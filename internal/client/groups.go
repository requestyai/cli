package client

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// GroupRole is the standing a member has within a group.
type GroupRole string

const (
	GroupRoleAdmin  GroupRole = "admin"
	GroupRoleMember GroupRole = "member"
)

// Group is a group as it appears in the organization listing.
type Group struct {
	ID                   string           `json:"id"`
	OrganizationID       string           `json:"organization_id"`
	Name                 string           `json:"name"`
	MembersCount         int              `json:"members_count"`
	MonthlySpend         decimal.Decimal  `json:"monthly_spend"`
	MonthlyLimit         *decimal.Decimal `json:"monthly_limit,omitempty"`
	MonthlyBudget        *decimal.Decimal `json:"monthly_budget,omitempty"`
	MonthlyBudgetPerUser *decimal.Decimal `json:"monthly_budget_per_user,omitempty"`
	CreatedBy            string           `json:"created_by"`
	CreatedAt            time.Time        `json:"created_at"`
}

// GroupDetails is a single group. The listing does not report members, so this
// is a wider record than Group rather than the same one.
type GroupDetails struct {
	ID                   string           `json:"id"`
	OrganizationID       string           `json:"organization_id"`
	Name                 string           `json:"name"`
	MonthlyLimit         *decimal.Decimal `json:"monthly_limit,omitempty"`
	MonthlyBudget        *decimal.Decimal `json:"monthly_budget,omitempty"`
	MonthlyBudgetPerUser *decimal.Decimal `json:"monthly_budget_per_user,omitempty"`
	CreatedBy            string           `json:"created_by"`
	CreatedAt            time.Time        `json:"created_at"`
	Members              []GroupMember    `json:"members"`
}

// GroupMember is one person in a group. A nil MonthlyLimit or RateLimit means
// the member is not capped beyond whatever the group and organization impose.
type GroupMember struct {
	ID           string           `json:"id"`
	Email        string           `json:"email"`
	Role         GroupRole        `json:"role"`
	MonthlyLimit *decimal.Decimal `json:"monthly_limit,omitempty"`
	MonthlySpend decimal.Decimal  `json:"monthly_spend"`
	RateLimit    *int64           `json:"rate_limit,omitempty"`
	Active       bool             `json:"active"`
}

// CreateGroupInput describes a group to create. A nil MonthlyLimit leaves the
// organization default in place.
type CreateGroupInput struct {
	Name         string
	MonthlyLimit *decimal.Decimal
}

// CreatedGroup identifies a group that was just created.
type CreatedGroup struct {
	ID string `json:"group_id"`
}

// Groups lists the organization's groups.
func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	endpoint, err := c.manageURL("group")
	if err != nil {
		return nil, err
	}

	var response struct {
		Groups []Group `json:"groups"`
	}
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return nil, err
	}

	return response.Groups, nil
}

// Group returns one group along with its members.
func (c *Client) Group(ctx context.Context, id string) (GroupDetails, error) {
	endpoint, err := c.manageURL("group", id)
	if err != nil {
		return GroupDetails{}, err
	}

	var response struct {
		Group GroupDetails `json:"group"`
	}
	if err := c.do(ctx, http.MethodGet, endpoint, nil, &response); err != nil {
		return GroupDetails{}, err
	}

	return response.Group, nil
}

// CreateGroup adds a group to the organization. In Group Budget mode the limit
// caps the group as a whole; in Global mode it caps each member separately.
func (c *Client) CreateGroup(ctx context.Context, input CreateGroupInput) (CreatedGroup, error) {
	endpoint, err := c.manageURL("group")
	if err != nil {
		return CreatedGroup{}, err
	}

	// The group endpoints take the limit as a JSON number, where the API key
	// endpoints take a string.
	body := struct {
		Name         string       `json:"name"`
		MonthlyLimit *json.Number `json:"monthly_limit,omitempty"`
	}{Name: input.Name}
	if input.MonthlyLimit != nil {
		limit := json.Number(input.MonthlyLimit.String())
		body.MonthlyLimit = &limit
	}

	var created CreatedGroup
	if err := c.do(ctx, http.MethodPost, endpoint, body, &created); err != nil {
		return CreatedGroup{}, err
	}

	return created, nil
}

// DeleteGroup removes a group for good. Its members keep their accounts.
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	endpoint, err := c.manageURL("group", id)
	if err != nil {
		return err
	}

	return c.do(ctx, http.MethodDelete, endpoint, nil, nil)
}

// AddGroupMember puts an existing organization user into a group.
func (c *Client) AddGroupMember(ctx context.Context, groupID, userID string, role GroupRole) error {
	endpoint, err := c.manageURL("group", groupID, "member")
	if err != nil {
		return err
	}

	body := struct {
		UserID string    `json:"user_id"`
		Role   GroupRole `json:"role"`
	}{UserID: userID, Role: role}

	return c.do(ctx, http.MethodPost, endpoint, body, nil)
}

// UpdateGroupMemberRole changes what a member may do within the group.
func (c *Client) UpdateGroupMemberRole(ctx context.Context, groupID, userID string, role GroupRole) error {
	endpoint, err := c.manageURL("group", groupID, "member", userID, "role")
	if err != nil {
		return err
	}

	body := struct {
		Role GroupRole `json:"role"`
	}{Role: role}

	return c.do(ctx, http.MethodPut, endpoint, body, nil)
}

// RemoveGroupMember takes a member out of a group, leaving the user in place.
func (c *Client) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	endpoint, err := c.manageURL("group", groupID, "member")
	if err != nil {
		return err
	}

	body := struct {
		UserID string `json:"user_id"`
	}{UserID: userID}

	return c.do(ctx, http.MethodDelete, endpoint, body, nil)
}
