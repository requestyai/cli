package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientAccessLists(t *testing.T) {
	reply := `{"access_lists":[` +
		`{"id":"list-1","name":"Production","organization_id":"org-123","auto_approve":false,` +
		`"chat":["openai/gpt-4o","anthropic/claude-sonnet-4-20250514"],"embedding":["openai/text-embedding-3-small"],` +
		`"image":[],"transcription":[],"speech":[]},` +
		`{"id":"list-2","name":"Everything new","organization_id":"org-123","auto_approve":true}]}`
	client, seen := newTestClient(t, http.StatusOK, reply)

	lists, err := client.AccessLists(context.Background())

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, seen.method)
	assert.Equal(t, "/v1/manage/access-list", seen.path)
	assert.Equal(t, "Bearer test-key", seen.auth)
	assert.Equal(t, []AccessList{
		{
			ID:             "list-1",
			Name:           "Production",
			OrganizationID: "org-123",
			Chat:           []string{"openai/gpt-4o", "anthropic/claude-sonnet-4-20250514"},
			Embedding:      []string{"openai/text-embedding-3-small"},
			Image:          []string{},
			Transcription:  []string{},
			Speech:         []string{},
		},
		{
			ID:             "list-2",
			Name:           "Everything new",
			OrganizationID: "org-123",
			AutoApprove:    true,
		},
	}, lists)
}

func TestClientAccessList(t *testing.T) {
	reply := `{"access_list":{"id":"list-1","name":"Production","organization_id":"org-123","auto_approve":true,` +
		`"chat":["openai/gpt-4o"],"embedding":[],"image":["openai/dall-e-3"],"transcription":["openai/whisper-1"],` +
		`"speech":["openai/tts-1"]}}`
	client, seen := newTestClient(t, http.StatusOK, reply)

	list, err := client.AccessList(context.Background(), "list-1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, seen.method)
	assert.Equal(t, "/v1/manage/access-list/list-1", seen.path)
	assert.Equal(t, AccessList{
		ID:             "list-1",
		Name:           "Production",
		OrganizationID: "org-123",
		AutoApprove:    true,
		Chat:           []string{"openai/gpt-4o"},
		Embedding:      []string{},
		Image:          []string{"openai/dall-e-3"},
		Transcription:  []string{"openai/whisper-1"},
		Speech:         []string{"openai/tts-1"},
	}, list)
}

func TestClientAccessListEscapesID(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"access_list":{"id":"list-1"}}`)

	_, err := client.AccessList(context.Background(), "../apikey")

	require.NoError(t, err)
	assert.Equal(t, "/v1/manage/access-list/..%2Fapikey", seen.path)
}

func TestAccessListModels(t *testing.T) {
	list := AccessList{
		Chat:          []string{"chat-model"},
		Embedding:     []string{"embedding-model"},
		Image:         []string{"image-model"},
		Transcription: []string{"transcription-model"},
		Speech:        []string{"speech-model"},
	}

	for _, modality := range Modalities {
		assert.Equal(t, []string{string(modality) + "-model"}, list.Models(modality), string(modality))
	}
	assert.Nil(t, list.Models("video"))
}

func TestClientCreateAccessList(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"id":"list-1"}`)

	created, err := client.CreateAccessList(context.Background(), CreateAccessListInput{
		Name:        "Production",
		AutoApprove: true,
		Chat:        []string{"openai/gpt-4o", "anthropic/claude-sonnet-4-20250514"},
		Speech:      []string{"openai/tts-1"},
	})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, seen.method)
	assert.Equal(t, "/v1/manage/access-list", seen.path)
	// Modalities with no models stay out of the request rather than going as
	// null, and auto_approve always goes so the default is explicit.
	assert.JSONEq(t, `{"name":"Production","auto_approve":true,`+
		`"chat":["openai/gpt-4o","anthropic/claude-sonnet-4-20250514"],"speech":["openai/tts-1"]}`, seen.body)
	assert.Equal(t, CreatedAccessList{ID: "list-1"}, created)
}

func TestClientCreateAccessListWithNameOnly(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"id":"list-1"}`)

	_, err := client.CreateAccessList(context.Background(), CreateAccessListInput{Name: "Empty"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Empty","auto_approve":false}`, seen.body)
}

func TestClientUpdateAccessListName(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, "")

	err := client.UpdateAccessListName(context.Background(), "list-1", "Staging")

	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, seen.method)
	assert.Equal(t, "/v1/manage/access-list/list-1", seen.path)
	assert.JSONEq(t, `{"name":"Staging"}`, seen.body)
}

func TestClientUpdateAccessListAutoApprove(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, "")

	err := client.UpdateAccessListAutoApprove(context.Background(), "list-1", false)

	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, seen.method)
	assert.Equal(t, "/v1/manage/access-list/list-1", seen.path)
	// false must still go out; leaving it off would leave the setting alone.
	assert.JSONEq(t, `{"auto_approve":false}`, seen.body)
}

func TestClientUpdateAccessListModels(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, "")

	err := client.UpdateAccessListModels(context.Background(), "list-1", map[Modality][]string{
		ModalityChat:  {"openai/gpt-4o"},
		ModalityImage: nil,
	})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPatch, seen.method)
	assert.Equal(t, "/v1/manage/access-list/list-1", seen.path)
	// A modality being cleared goes as an empty array, not null, and the
	// modalities not mentioned stay out of the request.
	assert.JSONEq(t, `{"chat":["openai/gpt-4o"],"image":[]}`, seen.body)
}

func TestClientDeleteAccessList(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, "")

	err := client.DeleteAccessList(context.Background(), "list-1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, seen.method)
	assert.Equal(t, "/v1/manage/access-list/list-1", seen.path)
	assert.Empty(t, seen.body)
}

func TestClientDeleteAccessListInUse(t *testing.T) {
	client, _ := newTestClient(t, http.StatusConflict,
		`{"error":{"origin":"router","message":"access list is assigned to 2 groups"}}`)

	err := client.DeleteAccessList(context.Background(), "list-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "access list is assigned to 2 groups")
	assert.Contains(t, err.Error(), "409")
}
