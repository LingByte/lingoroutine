package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	bailian "github.com/alibabacloud-go/bailian-20231229/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"
)

const (
	// DefaultBailianEndpoint is the public Bailian OpenAPI host (Beijing region).
	DefaultBailianEndpoint = "bailian.cn-beijing.aliyuncs.com"
	bailianSourceTypeFile  = "DATA_CENTER_FILE"
)

// AliyunHandler calls Alibaba Cloud Model Studio (百炼) knowledge-base OpenAPI.
// Credentials are AccessKey ID/Secret (main or RAM user with Bailian data permissions).
//
// Index selection: AliyunOptions.IndexID is the default knowledge base id. Per-call
// options.Namespace (UpsertOptions, QueryOptions, …) overrides IndexID when set.
//
// Upsert: submits SubmitIndexAddDocumentsJob. Each Record must reference an existing
// Data Center document id in Record.ID or metadata keys document_id / bailian_document_id.
//
// Query: maps to Retrieve (server-side dense retrieval + rerank). No local Embedder.
//
// List: ListIndexDocuments; ListOptions.Offset is a 1-based page number string.
//
// Get: scans ListIndexDocuments pages until all requested ids are found (bounded pages).
type AliyunHandler struct {
	client *bailian.Client

	WorkspaceID string
	IndexID     string
}

func (h *AliyunHandler) Provider() string {
	return KnowledgeAliyun
}

func (h *AliyunHandler) Upsert(ctx context.Context, records []Record, options *UpsertOptions) error {
	if h == nil || h.client == nil {
		return errors.New("handler is nil")
	}
	if len(records) == 0 {
		return nil
	}
	indexID := h.resolveIndexID(options)
	if indexID == "" {
		return errors.New("IndexID is required (AliyunOptions.IndexID or UpsertOptions.Namespace)")
	}
	if h.WorkspaceID == "" {
		return errors.New("WorkspaceID is required")
	}

	docIDs := make([]*string, 0, len(records))
	for _, r := range records {
		id := strings.TrimSpace(r.ID)
		if id == "" && r.Metadata != nil {
			if v, ok := r.Metadata["document_id"].(string); ok {
				id = strings.TrimSpace(v)
			}
			if id == "" {
				if v, ok := r.Metadata["bailian_document_id"].(string); ok {
					id = strings.TrimSpace(v)
				}
			}
		}
		if id == "" {
			return errors.New("each record needs Bailian document id in ID or metadata.document_id")
		}
		docIDs = append(docIDs, tea.String(id))
	}

	req := &bailian.SubmitIndexAddDocumentsJobRequest{
		IndexId:     tea.String(indexID),
		SourceType:  tea.String(bailianSourceTypeFile),
		DocumentIds: docIDs,
	}
	resp, err := h.client.SubmitIndexAddDocumentsJob(tea.String(h.WorkspaceID), req)
	if err != nil {
		return err
	}
	return checkSubmitAddDocsResponse(resp.Body)
}

func (h *AliyunHandler) Query(ctx context.Context, text string, options *QueryOptions) ([]QueryResult, error) {
	if h == nil || h.client == nil {
		return nil, errors.New("handler is nil")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("text is empty")
	}
	indexID := h.resolveIndexIDFromQuery(options)
	if indexID == "" {
		return nil, errors.New("IndexID is required")
	}
	if h.WorkspaceID == "" {
		return nil, errors.New("WorkspaceID is required")
	}

	topK := int32(5)
	if options != nil && options.TopK > 0 && options.TopK <= 100 {
		topK = int32(options.TopK)
	}
	rerankN := topK
	if rerankN > 20 {
		rerankN = 20
	}

	req := &bailian.RetrieveRequest{
		IndexId:              tea.String(indexID),
		Query:                tea.String(text),
		DenseSimilarityTopK:  tea.Int32(topK),
		SparseSimilarityTopK: tea.Int32(0),
		EnableReranking:      tea.Bool(true),
		RerankTopN:           tea.Int32(rerankN),
	}
	if err := applyBailianSearchFilters(req, options); err != nil {
		return nil, err
	}

	resp, err := h.client.Retrieve(tea.String(h.WorkspaceID), req)
	if err != nil {
		return nil, err
	}
	if err := checkRetrieveResponse(resp.Body); err != nil {
		return nil, err
	}
	if resp.Body == nil || resp.Body.Data == nil {
		return nil, nil
	}
	nodes := resp.Body.Data.Nodes
	out := make([]QueryResult, 0, len(nodes))
	for _, n := range nodes {
		if n == nil {
			continue
		}
		score := 0.0
		if n.Score != nil {
			score = *n.Score
		}
		txt := ""
		if n.Text != nil {
			txt = *n.Text
		}
		out = append(out, QueryResult{
			Record: recordFromBailianNode(txt, n.Metadata),
			Score:  score,
		})
	}
	return out, nil
}

func (h *AliyunHandler) Get(ctx context.Context, ids []string, options *GetOptions) ([]Record, error) {
	if h == nil || h.client == nil {
		return nil, errors.New("handler is nil")
	}
	if len(ids) == 0 {
		return []Record{}, nil
	}
	indexID := h.resolveIndexIDFromGet(options)
	if indexID == "" {
		return nil, errors.New("IndexID is required")
	}
	if h.WorkspaceID == "" {
		return nil, errors.New("WorkspaceID is required")
	}

	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			want[id] = struct{}{}
		}
	}

	const maxPages = 50
	pageSize := int32(50)
	found := make(map[string]Record)

	for page := int32(1); page <= maxPages && len(found) < len(want); page++ {
		listReq := &bailian.ListIndexDocumentsRequest{
			IndexId:    tea.String(indexID),
			PageNumber: tea.Int32(page),
			PageSize:   tea.Int32(pageSize),
		}
		resp, err := h.client.ListIndexDocuments(tea.String(h.WorkspaceID), listReq)
		if err != nil {
			return nil, err
		}
		if err := checkListDocumentsResponse(resp.Body); err != nil {
			return nil, err
		}
		if resp.Body == nil || resp.Body.Data == nil {
			break
		}
		docs := resp.Body.Data.Documents
		if len(docs) == 0 {
			break
		}
		for _, d := range docs {
			if d == nil || d.Id == nil {
				continue
			}
			id := strings.TrimSpace(*d.Id)
			if _, ok := want[id]; !ok {
				continue
			}
			if _, done := found[id]; done {
				continue
			}
			found[id] = recordFromBailianDocument(d)
		}
		if int64(len(docs)) < int64(pageSize) {
			break
		}
	}

	out := make([]Record, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if r, ok := found[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (h *AliyunHandler) List(ctx context.Context, options *ListOptions) (*ListResult, error) {
	if h == nil || h.client == nil {
		return nil, errors.New("handler is nil")
	}
	indexID := h.resolveIndexIDFromList(options)
	if indexID == "" {
		return nil, errors.New("IndexID is required")
	}
	if h.WorkspaceID == "" {
		return nil, errors.New("WorkspaceID is required")
	}

	page := int32(1)
	if options != nil {
		if p, err := strconv.ParseInt(strings.TrimSpace(options.Offset), 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}
	limit := int32(20)
	if options != nil && options.Limit > 0 {
		limit = int32(options.Limit)
	}

	req := &bailian.ListIndexDocumentsRequest{
		IndexId:    tea.String(indexID),
		PageNumber: tea.Int32(page),
		PageSize:   tea.Int32(limit),
	}
	if options != nil && options.Filters != nil {
		if v, ok := options.Filters["document_status"].(string); ok && strings.TrimSpace(v) != "" {
			req.DocumentStatus = tea.String(strings.TrimSpace(v))
		}
		if v, ok := options.Filters["document_name"].(string); ok && strings.TrimSpace(v) != "" {
			req.DocumentName = tea.String(strings.TrimSpace(v))
		}
	}

	resp, err := h.client.ListIndexDocuments(tea.String(h.WorkspaceID), req)
	if err != nil {
		return nil, err
	}
	if err := checkListDocumentsResponse(resp.Body); err != nil {
		return nil, err
	}
	if resp.Body == nil || resp.Body.Data == nil {
		return &ListResult{}, nil
	}
	docs := resp.Body.Data.Documents
	recs := make([]Record, 0, len(docs))
	for _, d := range docs {
		if d == nil {
			continue
		}
		recs = append(recs, recordFromBailianDocument(d))
	}

	next := ""
	if len(docs) > 0 && int64(len(docs)) >= int64(limit) {
		next = strconv.FormatInt(int64(page+1), 10)
	}
	return &ListResult{Records: recs, NextOffset: next}, nil
}

func (h *AliyunHandler) Delete(ctx context.Context, ids []string, options *DeleteOptions) error {
	if h == nil || h.client == nil {
		return errors.New("handler is nil")
	}
	if len(ids) == 0 {
		return nil
	}
	indexID := h.resolveIndexIDFromDelete(options)
	if indexID == "" {
		return errors.New("IndexID is required")
	}
	if h.WorkspaceID == "" {
		return errors.New("WorkspaceID is required")
	}

	docIDs := make([]*string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			docIDs = append(docIDs, tea.String(id))
		}
	}
	if len(docIDs) == 0 {
		return nil
	}

	req := &bailian.DeleteIndexDocumentRequest{
		IndexId:     tea.String(indexID),
		DocumentIds: docIDs,
	}
	resp, err := h.client.DeleteIndexDocument(tea.String(h.WorkspaceID), req)
	if err != nil {
		return err
	}
	return checkDeleteDocumentsResponse(resp.Body)
}

func newAliyunHandler(opts *AliyunOptions) (*AliyunHandler, error) {
	if opts == nil {
		return nil, errors.New("aliyun options are required")
	}
	ak := strings.TrimSpace(opts.AccessKeyID)
	sk := strings.TrimSpace(opts.AccessKeySecret)
	if ak == "" || sk == "" {
		return nil, errors.New("AccessKeyID and AccessKeySecret are required")
	}
	ws := strings.TrimSpace(opts.WorkspaceID)
	if ws == "" {
		return nil, errors.New("WorkspaceID is required")
	}
	idx := strings.TrimSpace(opts.IndexID)
	if idx == "" {
		return nil, errors.New("IndexID is required (Bailian knowledge base id)")
	}

	endpoint := strings.TrimSpace(opts.Endpoint)
	if endpoint == "" {
		endpoint = DefaultBailianEndpoint
	}
	region := strings.TrimSpace(opts.RegionID)
	if region == "" {
		region = "cn-beijing"
	}

	cfg := &openapi.Config{
		AccessKeyId:     tea.String(ak),
		AccessKeySecret: tea.String(sk),
		Endpoint:        tea.String(endpoint),
		RegionId:        tea.String(region),
	}
	cli, err := bailian.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &AliyunHandler{
		client:      cli,
		WorkspaceID: ws,
		IndexID:     idx,
	}, nil
}

func (h *AliyunHandler) resolveIndexID(opt *UpsertOptions) string {
	if opt != nil && strings.TrimSpace(opt.Namespace) != "" {
		return strings.TrimSpace(opt.Namespace)
	}
	return h.IndexID
}

func (h *AliyunHandler) resolveIndexIDFromQuery(opt *QueryOptions) string {
	if opt != nil && strings.TrimSpace(opt.Namespace) != "" {
		return strings.TrimSpace(opt.Namespace)
	}
	return h.IndexID
}

func (h *AliyunHandler) resolveIndexIDFromGet(opt *GetOptions) string {
	if opt != nil && strings.TrimSpace(opt.Namespace) != "" {
		return strings.TrimSpace(opt.Namespace)
	}
	return h.IndexID
}

func (h *AliyunHandler) resolveIndexIDFromList(opt *ListOptions) string {
	if opt != nil && strings.TrimSpace(opt.Namespace) != "" {
		return strings.TrimSpace(opt.Namespace)
	}
	return h.IndexID
}

func (h *AliyunHandler) resolveIndexIDFromDelete(opt *DeleteOptions) string {
	if opt != nil && strings.TrimSpace(opt.Namespace) != "" {
		return strings.TrimSpace(opt.Namespace)
	}
	return h.IndexID
}

func checkSubmitAddDocsResponse(body *bailian.SubmitIndexAddDocumentsJobResponseBody) error {
	if body == nil {
		return errors.New("SubmitIndexAddDocumentsJob: empty response body")
	}
	if tea.BoolValue(body.Success) {
		return nil
	}
	return bailianFail("SubmitIndexAddDocumentsJob", body.Code, body.Message)
}

func checkRetrieveResponse(body *bailian.RetrieveResponseBody) error {
	if body == nil {
		return errors.New("Retrieve: empty response body")
	}
	if tea.BoolValue(body.Success) {
		return nil
	}
	return bailianFail("Retrieve", body.Code, body.Message)
}

func checkListDocumentsResponse(body *bailian.ListIndexDocumentsResponseBody) error {
	if body == nil {
		return errors.New("ListIndexDocuments: empty response body")
	}
	if tea.BoolValue(body.Success) {
		return nil
	}
	return bailianFail("ListIndexDocuments", body.Code, body.Message)
}

func checkDeleteDocumentsResponse(body *bailian.DeleteIndexDocumentResponseBody) error {
	if body == nil {
		return errors.New("DeleteIndexDocument: empty response body")
	}
	if tea.BoolValue(body.Success) {
		return nil
	}
	return bailianFail("DeleteIndexDocument", body.Code, body.Message)
}

func bailianFail(op string, code *string, msg *string) error {
	c, m := "", ""
	if code != nil {
		c = *code
	}
	if msg != nil {
		m = *msg
	}
	return fmt.Errorf("%s: bailian api failed code=%s message=%s", op, c, m)
}

func applyBailianSearchFilters(req *bailian.RetrieveRequest, options *QueryOptions) error {
	if options == nil || options.Filters == nil {
		return nil
	}
	raw, ok := options.Filters["search_filters"]
	if !ok || raw == nil {
		return nil
	}
	var filters []map[string]*string
	switch v := raw.(type) {
	case []map[string]*string:
		filters = v
	case []map[string]string:
		for _, m := range v {
			row := make(map[string]*string, len(m))
			for k, val := range m {
				row[k] = tea.String(val)
			}
			filters = append(filters, row)
		}
	case string:
		if err := json.Unmarshal([]byte(v), &filters); err != nil {
			return fmt.Errorf("decode search_filters json: %w", err)
		}
	case []byte:
		if err := json.Unmarshal(v, &filters); err != nil {
			return fmt.Errorf("decode search_filters json: %w", err)
		}
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("search_filters must be []map[string]*string or JSON string")
		}
		if err := json.Unmarshal(b, &filters); err != nil {
			return fmt.Errorf("decode search_filters: %w", err)
		}
	}
	req.SearchFilters = filters
	return nil
}

func recordFromBailianNode(text string, meta interface{}) Record {
	rec := Record{Content: text, Metadata: metadataToMap(meta)}
	if rec.Metadata != nil {
		if v, ok := rec.Metadata["title"].(string); ok {
			rec.Title = v
		}
		if v, ok := rec.Metadata["hier_title"].(string); ok && rec.Title == "" {
			rec.Title = v
		}
		if v, ok := rec.Metadata["doc_id"].(string); ok {
			rec.ID = v
		}
		if v, ok := rec.Metadata["doc_name"].(string); ok {
			rec.Source = v
		}
	}
	return rec
}

func recordFromBailianDocument(d *bailian.ListIndexDocumentsResponseBodyDataDocuments) Record {
	rec := Record{Metadata: map[string]any{}}
	if d.Id != nil {
		rec.ID = *d.Id
	}
	if d.Name != nil {
		rec.Title = *d.Name
		rec.Source = *d.Name
	}
	if d.DocumentType != nil {
		rec.Metadata["document_type"] = *d.DocumentType
	}
	if d.Status != nil {
		rec.Metadata["document_status"] = *d.Status
	}
	if d.SourceId != nil {
		rec.Metadata["source_id"] = *d.SourceId
	}
	if d.Size != nil {
		rec.Metadata["size"] = *d.Size
	}
	return rec
}

func metadataToMap(v interface{}) map[string]any {
	if v == nil {
		return nil
	}
	switch m := v.(type) {
	case map[string]any:
		return m
	default:
		b, err := json.Marshal(m)
		if err != nil {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal(b, &out); err != nil {
			return nil
		}
		return out
	}
}
