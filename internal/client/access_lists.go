package client

import (
	"context"
	"net/http"
)

// Modality is one kind of model an access list can allow.
type Modality string

const (
	ModalityChat          Modality = "chat"
	ModalityEmbedding     Modality = "embedding"
	ModalityImage         Modality = "image"
	ModalityTranscription Modality = "transcription"
	ModalitySpeech        Modality = "speech"
)

// Modalities lists every modality in the order the API documents them.
var Modalities = []Modality{
	ModalityChat,
	ModalityEmbedding,
	ModalityImage,
	ModalityTranscription,
	ModalitySpeech,
}

// AccessList is a named set of models that groups and API keys may be limited
// to. Only the models listed are allowed, whatever their modality, so a list
// with only chat models blocks embeddings, images and audio outright.
type AccessList struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	OrganizationID string   `json:"organization_id"`
	AutoApprove    bool     `json:"auto_approve"`
	Chat           []string `json:"chat"`
	Embedding      []string `json:"embedding"`
	Image          []string `json:"image"`
	Transcription  []string `json:"transcription"`
	Speech         []string `json:"speech"`
}

// Models returns the allowed models of one modality.
func (list AccessList) Models(modality Modality) []string {
	switch modality {
	case ModalityChat:
		return list.Chat
	case ModalityEmbedding:
		return list.Embedding
	case ModalityImage:
		return list.Image
	case ModalityTranscription:
		return list.Transcription
	case ModalitySpeech:
		return list.Speech
	default:
		return nil
	}
}

// CreateAccessListInput describes an access list to create. A modality left
// nil allows no models of that kind.
type CreateAccessListInput struct {
	Name          string
	AutoApprove   bool
	Chat          []string
	Embedding     []string
	Image         []string
	Transcription []string
	Speech        []string
}

// CreatedAccessList identifies an access list that was just created.
type CreatedAccessList struct {
	ID string `json:"id"`
}

// AccessLists lists the organization's access lists.
func (c *Client) AccessLists(ctx context.Context) ([]AccessList, error) {
	endpoint, err := c.manageURL("access-list")
	if err != nil {
		return nil, err
	}

	var response struct {
		AccessLists []AccessList `json:"access_lists"`
	}
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response.AccessLists, nil
}

// AccessList returns one access list along with the models it allows.
func (c *Client) AccessList(ctx context.Context, id string) (AccessList, error) {
	endpoint, err := c.manageURL("access-list", id)
	if err != nil {
		return AccessList{}, err
	}

	var response struct {
		AccessList AccessList `json:"access_list"`
	}
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return AccessList{}, err
	}

	return response.AccessList, nil
}

// CreateAccessList adds an access list to the organization.
func (c *Client) CreateAccessList(ctx context.Context, input CreateAccessListInput) (CreatedAccessList, error) {
	endpoint, err := c.manageURL("access-list")
	if err != nil {
		return CreatedAccessList{}, err
	}

	body := struct {
		Name          string   `json:"name"`
		AutoApprove   bool     `json:"auto_approve"`
		Chat          []string `json:"chat,omitempty"`
		Embedding     []string `json:"embedding,omitempty"`
		Image         []string `json:"image,omitempty"`
		Transcription []string `json:"transcription,omitempty"`
		Speech        []string `json:"speech,omitempty"`
	}{
		Name:          input.Name,
		AutoApprove:   input.AutoApprove,
		Chat:          input.Chat,
		Embedding:     input.Embedding,
		Image:         input.Image,
		Transcription: input.Transcription,
		Speech:        input.Speech,
	}

	var created CreatedAccessList
	if err := c.do(ctx, http.MethodPost, endpoint, body, &created); err != nil {
		return CreatedAccessList{}, err
	}

	return created, nil
}

// UpdateAccessListName renames an access list.
func (c *Client) UpdateAccessListName(ctx context.Context, id, name string) error {
	body := struct {
		Name string `json:"name"`
	}{Name: name}

	return c.patchAccessList(ctx, id, body)
}

// UpdateAccessListAutoApprove decides whether new models matching the list are
// approved without anyone adding them by hand.
func (c *Client) UpdateAccessListAutoApprove(ctx context.Context, id string, autoApprove bool) error {
	body := struct {
		AutoApprove bool `json:"auto_approve"`
	}{AutoApprove: autoApprove}

	return c.patchAccessList(ctx, id, body)
}

// UpdateAccessListModels replaces the allowed models of each modality given.
// A nil or empty list removes every model of that modality; modalities not in
// the map are left as they are.
func (c *Client) UpdateAccessListModels(ctx context.Context, id string, models map[Modality][]string) error {
	body := make(map[Modality][]string, len(models))
	for modality, allowed := range models {
		if allowed == nil {
			// Encode as [] rather than null, which is what the API takes to
			// mean "no models".
			allowed = []string{}
		}
		body[modality] = allowed
	}

	return c.patchAccessList(ctx, id, body)
}

// patchAccessList sends a partial update; only the fields in body change.
func (c *Client) patchAccessList(ctx context.Context, id string, body any) error {
	endpoint, err := c.manageURL("access-list", id)
	if err != nil {
		return err
	}

	return c.do(ctx, http.MethodPatch, endpoint, body, nil)
}

// DeleteAccessList removes an access list for good. The API refuses while the
// list is still attached to a group or API key.
func (c *Client) DeleteAccessList(ctx context.Context, id string) error {
	endpoint, err := c.manageURL("access-list", id)
	if err != nil {
		return err
	}

	return c.do(ctx, http.MethodDelete, endpoint, nil, nil)
}
