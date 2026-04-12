package knowledge

import (
	"errors"
	"net/http"
	"strings"
)

var ErrUnsupportedKnowledgeProvider = errors.New("unsupported knowledge provider")

type QdrantOptions struct {
	BaseURL    string
	APIKey     string
	Collection string
	HTTPClient *http.Client
	Embedder   Embedder
}

// AliyunOptions configures Alibaba Cloud Model Studio (百炼) knowledge access.
// Use AccessKey pair with Bailian data permissions (e.g. AliyunBailianDataFullAccess for RAM users).
type AliyunOptions struct {
	AccessKeyID     string
	AccessKeySecret string
	// Endpoint defaults to DefaultBailianEndpoint (Beijing public endpoint).
	Endpoint string
	// RegionID defaults to cn-beijing when Endpoint is empty.
	RegionID string
	// WorkspaceID is the business space id from the Bailian console.
	WorkspaceID string
	// IndexID is the knowledge base id (CreateIndex / console).
	IndexID string
}

type FactoryOptions struct {
	Qdrant *QdrantOptions
	Aliyun *AliyunOptions
}

func New(provider string, opts *FactoryOptions) (KnowledgeHandler, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if p == "" {
		p = KnowledgeQdrant
	}

	switch p {
	case KnowledgeQdrant:
		if opts == nil || opts.Qdrant == nil {
			return nil, errors.New("qdrant options are required")
		}
		q := opts.Qdrant
		return &QdrantHandler{BaseURL: q.BaseURL, APIKey: q.APIKey, Collection: q.Collection, HTTPClient: q.HTTPClient, Embedder: q.Embedder}, nil
	case KnowledgeAliyun:
		if opts == nil || opts.Aliyun == nil {
			return nil, errors.New("aliyun options are required")
		}
		return newAliyunHandler(opts.Aliyun)
	default:
		return nil, ErrUnsupportedKnowledgeProvider
	}
}
