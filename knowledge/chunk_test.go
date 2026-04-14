package knowledge

import (
	"context"
	"testing"

	"github.com/LingByte/lingoroutine/llm"
	"github.com/LingByte/lingoroutine/utils"
	"github.com/stretchr/testify/assert"
)

type fakeChunkLLM struct {
	resp string
	err  error
}

func (f *fakeChunkLLM) Query(text, model string) (string, error) {
	_ = text
	_ = model
	return f.resp, f.err
}

func (f *fakeChunkLLM) Provider() string { return "fake" }
func (f *fakeChunkLLM) QueryWithOptions(text string, options *llm.QueryOptions) (*llm.QueryResponse, error) {
	_ = text
	_ = options
	return &llm.QueryResponse{Choices: []llm.QueryChoice{{Index: 0, Content: f.resp}}}, f.err
}
func (f *fakeChunkLLM) QueryStream(text string, options *llm.QueryOptions, callback func(segment string, isComplete bool) error) (*llm.QueryResponse, error) {
	_ = text
	_ = options
	if callback != nil {
		_ = callback(f.resp, false)
		_ = callback("", true)
	}
	return &llm.QueryResponse{Choices: []llm.QueryChoice{{Index: 0, Content: f.resp}}}, f.err
}
func (f *fakeChunkLLM) Interrupt() {}

var _ llm.LLMHandler = (*fakeChunkLLM)(nil)

func TestLLMChunker_Chunk_PureJSON(t *testing.T) {
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: `[{"title":"A","text":"hello"},{"title":"B","text":"world"}]`}}
	chunks, err := c.Chunk(context.Background(), "input", &ChunkOptions{MaxChars: 100})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(chunks))
	assert.Equal(t, "A", chunks[0].Title)
	assert.Equal(t, "hello", chunks[0].Text)
}

func TestLLMChunker_Chunk_FencedJSON(t *testing.T) {
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: "```json\n[{\"title\":\"A\",\"text\":\"hello\"}]\n```"}}
	chunks, err := c.Chunk(context.Background(), "input", &ChunkOptions{MaxChars: 100})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, "hello", chunks[0].Text)
}

func TestLLMChunker_Chunk_NoiseAroundJSON(t *testing.T) {
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: "Sure, here you go:\n[{\"title\":\"A\",\"text\":\"hello\"}]\nThanks"}}
	chunks, err := c.Chunk(context.Background(), "input", &ChunkOptions{MaxChars: 100})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, "A", chunks[0].Title)
}

func TestLLMChunker_Chunk_ChunksObject(t *testing.T) {
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: `{"chunks":[{"title":"S1","text":"one"},{"title":"S2","text":"two"}]}`}}
	chunks, err := c.Chunk(context.Background(), "input", &ChunkOptions{MaxChars: 100})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(chunks))
	assert.Equal(t, "one", chunks[0].Text)
}

func TestLLMChunker_Chunk_BracketInsideText(t *testing.T) {
	raw := `{"chunks":[{"title":"A","text":"see ] not end"}]}`
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: raw}}
	chunks, err := c.Chunk(context.Background(), "input", &ChunkOptions{MaxChars: 100})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
	assert.Equal(t, "see ] not end", chunks[0].Text)
}

func TestLLMChunker_Chunk_CleanTextInvalidUTF8(t *testing.T) {
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: `{"chunks":[{"title":"A","text":"ok"}]}`}}
	chunks, err := c.Chunk(context.Background(), "pre\xffpost", nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(chunks))
}

func TestLLMChunker_Chunk_PreChunkCleanOverride(t *testing.T) {
	c := &LLMChunker{LLM: &fakeChunkLLM{resp: `{"chunks":[{"title":"A","text":"ok"}]}`}}
	chunks, err := c.Chunk(context.Background(), "# H1\n\nbody", &ChunkOptions{
		MaxChars:      100,
		PreChunkClean: &utils.Options{},
	})
	assert.NoError(t, err)
	assert.Len(t, chunks, 1)
}
