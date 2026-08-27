package client

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ptr returns the address of value, for the fields the API may leave out.
func ptr[T any](value T) *T {
	return &value
}

func TestClientGroups(t *testing.T) {
	// The group endpoints report money as JSON numbers rather than strings, and
	// which budget field is filled in depends on the organization's budget mode.
	reply := `{"groups":[` +
		`{"id":"group-1","organization_id":"org-123","name":"Engineering Team","members_count":5,` +
		`"monthly_spend":450.25,"monthly_limit":1000,"created_by":"user-123",` +
		`"created_at":"2026-01-15T10:30:00Z"},` +
		`{"id":"group-2","organization_id":"org-123","name":"Research","members_count":0,` +
		`"monthly_spend":0,"monthly_limit":null,"monthly_budget":2000,"monthly_budget_per_user":250,` +
		`"created_by":"user-123","created_at":"2026-02-01T00:00:00Z"}]}`
	client, seen := newTestClient(t, http.StatusOK, reply)

	groups, err := client.Groups(context.Background())

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, seen.method)
	assert.Equal(t, "/v1/manage/group", seen.path)
	assert.Equal(t, "Bearer test-key", seen.auth)
	assert.Equal(t, []Group{
		{
			ID:             "group-1",
			OrganizationID: "org-123",
			Name:           "Engineering Team",
			MembersCount:   5,
			MonthlySpend:   decimal.RequireFromString("450.25"),
			MonthlyLimit:   ptr(decimal.RequireFromString("1000")),
			CreatedBy:      "user-123",
			CreatedAt:      time.Date(2026, time.January, 15, 10, 30, 0, 0, time.UTC),
		},
		{
			ID:                   "group-2",
			OrganizationID:       "org-123",
			Name:                 "Research",
			MonthlySpend:         decimal.RequireFromString("0"),
			MonthlyBudget:        ptr(decimal.RequireFromString("2000")),
			MonthlyBudgetPerUser: ptr(decimal.RequireFromString("250")),
			CreatedBy:            "user-123",
			CreatedAt:            time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
		},
	}, groups)
}

func TestClientGroup(t *testing.T) {
	reply := `{"group":{"id":"group-1","organization_id":"org-123","name":"Engineering Team",` +
		`"monthly_limit":1000,"created_by":"user-123","created_at":"2026-01-15T10:30:00Z","members":[` +
		`{"id":"user-1","email":"lead@example.com","role":"admin","monthly_limit":500,` +
		`"monthly_spend":250.5,"rate_limit":100,"active":true},` +
		`{"id":"user-2","email":"dev@example.com","role":"member","monthly_limit":null,` +
		`"monthly_spend":0,"rate_limit":null,"active":false}]}}`
	client, seen := newTestClient(t, http.StatusOK, reply)

	group, err := client.Group(context.Background(), "group-1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodGet, seen.method)
	assert.Equal(t, "/v1/manage/group/group-1", seen.path)
	assert.Equal(t, GroupDetails{
		ID:             "group-1",
		OrganizationID: "org-123",
		Name:           "Engineering Team",
		MonthlyLimit:   ptr(decimal.RequireFromString("1000")),
		CreatedBy:      "user-123",
		CreatedAt:      time.Date(2026, time.January, 15, 10, 30, 0, 0, time.UTC),
		Members: []GroupMember{
			{
				ID:           "user-1",
				Email:        "lead@example.com",
				Role:         GroupRoleAdmin,
				MonthlyLimit: ptr(decimal.RequireFromString("500")),
				MonthlySpend: decimal.RequireFromString("250.5"),
				RateLimit:    ptr(int64(100)),
				Active:       true,
			},
			{
				ID:           "user-2",
				Email:        "dev@example.com",
				Role:         GroupRoleMember,
				MonthlySpend: decimal.RequireFromString("0"),
			},
		},
	}, group)
}

func TestClientGroupEscapesID(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"group":{"id":"group-1"}}`)

	_, err := client.Group(context.Background(), "../apikey")

	require.NoError(t, err)
	assert.Equal(t, "/v1/manage/group/..%2Fapikey", seen.path)
}

func TestClientCreateGroup(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"group_id":"group-1"}`)

	limit := decimal.RequireFromString("1000")
	created, err := client.CreateGroup(context.Background(), CreateGroupInput{
		Name:         "Engineering Team",
		MonthlyLimit: &limit,
	})

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, seen.method)
	assert.Equal(t, "/v1/manage/group", seen.path)
	// The limit goes out as a number, which is what this endpoint accepts.
	assert.JSONEq(t, `{"name":"Engineering Team","monthly_limit":1000}`, seen.body)
	assert.Equal(t, CreatedGroup{ID: "group-1"}, created)
}

func TestClientCreateGroupOmitsUnsetLimit(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, `{"group_id":"group-1"}`)

	_, err := client.CreateGroup(context.Background(), CreateGroupInput{Name: "Engineering Team"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"Engineering Team"}`, seen.body)
}

func TestClientDeleteGroup(t *testing.T) {
	client, seen := newTestClient(t, http.StatusNoContent, "")

	err := client.DeleteGroup(context.Background(), "group-1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, seen.method)
	assert.Equal(t, "/v1/manage/group/group-1", seen.path)
	assert.Empty(t, seen.body)
}

func TestClientAddGroupMember(t *testing.T) {
	client, seen := newTestClient(t, http.StatusNoContent, "")

	err := client.AddGroupMember(context.Background(), "group-1", "user-1", GroupRoleMember)

	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, seen.method)
	assert.Equal(t, "/v1/manage/group/group-1/member", seen.path)
	assert.JSONEq(t, `{"user_id":"user-1","role":"member"}`, seen.body)
}

func TestClientUpdateGroupMemberRole(t *testing.T) {
	client, seen := newTestClient(t, http.StatusOK, "")

	err := client.UpdateGroupMemberRole(context.Background(), "group-1", "user-1", GroupRoleAdmin)

	require.NoError(t, err)
	assert.Equal(t, http.MethodPut, seen.method)
	assert.Equal(t, "/v1/manage/group/group-1/member/user-1/role", seen.path)
	assert.JSONEq(t, `{"role":"admin"}`, seen.body)
}

func TestClientRemoveGroupMember(t *testing.T) {
	client, seen := newTestClient(t, http.StatusNoContent, "")

	err := client.RemoveGroupMember(context.Background(), "group-1", "user-1")

	require.NoError(t, err)
	assert.Equal(t, http.MethodDelete, seen.method)
	assert.Equal(t, "/v1/manage/group/group-1/member", seen.path)
	assert.JSONEq(t, `{"user_id":"user-1"}`, seen.body)
}

func TestClientGroupReportsAPIError(t *testing.T) {
	client, _ := newTestClient(t, http.StatusNotFound, `{"error":{"origin":"router","message":"group not found"}}`)

	_, err := client.Group(context.Background(), "group-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "group not found")
	assert.Contains(t, err.Error(), "404")
}
