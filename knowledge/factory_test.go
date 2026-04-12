package knowledge

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFactory_DefaultProviderRequiresOptions(t *testing.T) {
	_, err := New("", nil)
	assert.Error(t, err)
}

func TestFactory_Qdrant(t *testing.T) {
	h, err := New("qdrant", &FactoryOptions{Qdrant: &QdrantOptions{BaseURL: "http://localhost:6333", Collection: "c", Embedder: &NvidiaEmbedClient{}}})
	assert.NoError(t, err)
	assert.NotNil(t, h)
	assert.Equal(t, KnowledgeQdrant, h.Provider())
}

func TestFactory_Unsupported(t *testing.T) {
	_, err := New("nope", &FactoryOptions{})
	assert.ErrorIs(t, err, ErrUnsupportedKnowledgeProvider)
}

func TestFactory_Aliyun(t *testing.T) {
	h, err := New(KnowledgeAliyun, &FactoryOptions{Aliyun: &AliyunOptions{
		AccessKeyID:     "test-ak",
		AccessKeySecret: "test-sk",
		WorkspaceID:     "llm-test-workspace",
		IndexID:         "test-index-id",
		Endpoint:        DefaultBailianEndpoint,
		RegionID:        "cn-beijing",
	}})
	assert.NoError(t, err)
	assert.NotNil(t, h)
	assert.Equal(t, KnowledgeAliyun, h.Provider())
}

func TestFactory_Aliyun_requiresOptions(t *testing.T) {
	_, err := New(KnowledgeAliyun, nil)
	assert.Error(t, err)
	_, err = New(KnowledgeAliyun, &FactoryOptions{})
	assert.Error(t, err)
}
