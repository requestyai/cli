package cmd

import (
	"cmp"
	"fmt"
	"strconv"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/util"
	"github.com/spf13/cobra"
)

const (
	groupNameFlag         = "name"
	groupMonthlyLimitFlag = "monthly-limit"
	groupRoleFlag         = "role"
	groupYesFlag          = "yes"
)

func newGroupsCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "groups",
		Aliases: []string{"group"},
		Short:   "Manage the groups in your organization",
		Long: "Manage the groups in your organization.\n\n" +
			"A group gathers users under a shared budget. Members are existing users of\n" +
			"your organization, referred to by their user id.",
	}

	cmd.PersistentFlags().Bool(jsonFlag, false, "print JSON instead of a table")
	cmd.AddCommand(
		newGroupsListCommand(env),
		newGroupsShowCommand(env),
		newGroupsCreateCommand(env),
		newGroupsDeleteCommand(env),
		newGroupsMembersCommand(env),
	)

	return cmd
}

func newGroupsListCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the groups in your organization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			groups, err := env.apiv2Client.Groups(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, groups)
			}
			if len(groups) == 0 {
				_, err := fmt.Fprintln(out, "No groups yet.")
				return err
			}

			rows := make([][]string, 0, len(groups))
			for _, group := range groups {
				rows = append(rows, []string{
					group.ID,
					group.Name,
					strconv.Itoa(group.MembersCount),
					formatMoney(group.MonthlySpend),
					// The budget mode decides which field carries the cap.
					formatOptionalLimit(cmp.Or(group.MonthlyLimit, group.MonthlyBudget)),
					formatDate(group.CreatedAt),
				})
			}

			return writeTable(out, []string{"ID", "NAME", "MEMBERS", "SPEND", "LIMIT", "CREATED"}, rows)
		},
	}
}

func newGroupsShowCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one group and its members",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseGroupID(args[0])
			if err != nil {
				return err
			}

			group, err := env.apiv2Client.Group(cmd.Context(), id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, group)
			}

			fields := [][2]string{
				{"ID", group.ID},
				{"Name", group.Name},
				{"Organization", group.OrganizationID},
			}
			fields = append(fields, groupBudgetFields(group)...)
			fields = append(fields,
				[2]string{"Created by", group.CreatedBy},
				[2]string{"Created at", formatTime(group.CreatedAt)},
				[2]string{"Members", strconv.Itoa(len(group.Members))},
			)

			if err := writeFields(out, fields); err != nil {
				return err
			}

			if len(group.Members) == 0 {
				return nil
			}

			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}

			rows := make([][]string, 0, len(group.Members))
			for _, member := range group.Members {
				rows = append(rows, []string{
					member.ID,
					member.Email,
					string(member.Role),
					formatMoney(member.MonthlySpend),
					formatOptionalLimit(member.MonthlyLimit),
					formatRateLimit(member.RateLimit),
					strconv.FormatBool(member.Active),
				})
			}

			return writeTable(out, []string{"MEMBER", "EMAIL", "ROLE", "SPEND", "LIMIT", "RATE LIMIT", "ACTIVE"}, rows)
		},
	}
}

// groupBudgetFields lists whichever budget the API reported. Global mode fills
// in a monthly limit while Group Budget mode fills in a budget for the group
// and one for each member, so report only what came back.
func groupBudgetFields(group client.GroupDetails) [][2]string {
	fields := make([][2]string, 0, 3)

	if group.MonthlyBudget != nil {
		fields = append(fields, [2]string{"Monthly budget", formatOptionalLimit(group.MonthlyBudget)})
	}
	if group.MonthlyBudgetPerUser != nil {
		fields = append(fields, [2]string{"Monthly budget per user", formatOptionalLimit(group.MonthlyBudgetPerUser)})
	}
	if group.MonthlyLimit != nil || len(fields) == 0 {
		fields = append(fields, [2]string{"Monthly limit", formatOptionalLimit(group.MonthlyLimit)})
	}

	return fields
}
func newGroupsCreateCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a group",
		Long: "Create a group.\n\n" +
			"How the monthly limit is applied depends on your organization's budget mode:\n" +
			"it caps the group as a whole in Group Budget mode, and each member of the\n" +
			"group separately in Global mode.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()

			name, err := flags.GetString(groupNameFlag)
			if err != nil {
				return err
			}

			input := client.CreateGroupInput{Name: name}

			if flags.Changed(groupMonthlyLimitFlag) {
				raw, err := flags.GetString(groupMonthlyLimitFlag)
				if err != nil {
					return err
				}
				limit, err := util.ParseMoney("--"+groupMonthlyLimitFlag, raw)
				if err != nil {
					return err
				}
				input.MonthlyLimit = &limit
			}

			created, err := env.apiv2Client.CreateGroup(cmd.Context(), input)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, created)
			}

			_, err = fmt.Fprintf(out, "Created group %s with id %s.\n", name, created.ID)
			return err
		},
	}

	flags := cmd.Flags()
	flags.String(groupNameFlag, "", "name for the new group")
	flags.String(groupMonthlyLimitFlag, "", "monthly spending cap in dollars, for example 1000 (default: the organization setting)")
	if err := cmd.MarkFlagRequired(groupNameFlag); err != nil {
		panic(err)
	}

	return cmd
}

func newGroupsDeleteCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a group",
		Long: "Delete a group.\n\n" +
			"Deletion is permanent. The members keep their accounts, but lose the budget\n" +
			"and the access the group gave them.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseGroupID(args[0])
			if err != nil {
				return err
			}

			skipPrompt, err := cmd.Flags().GetBool(groupYesFlag)
			if err != nil {
				return err
			}

			if !skipPrompt {
				confirmed, err := util.Confirm(
					cmd.InOrStdin(),
					cmd.OutOrStdout(),
					fmt.Sprintf("Delete group %s? This cannot be undone [y/N]: ", id),
				)
				if err != nil {
					return err
				}
				if !confirmed {
					_, err := fmt.Fprintln(cmd.OutOrStdout(), "Left it alone.")
					return err
				}
			}

			if err := env.apiv2Client.DeleteGroup(cmd.Context(), id); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, map[string]any{"id": id, "deleted": true})
			}

			_, err = fmt.Fprintf(out, "Deleted group %s.\n", id)
			return err
		},
	}

	cmd.Flags().BoolP(groupYesFlag, "y", false, "delete without asking first")

	return cmd
}

func newGroupsMembersCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "members",
		Aliases: []string{"member"},
		Short:   "Manage the members of a group",
		Long: "Manage the members of a group.\n\n" +
			"Members are existing users of your organization. Run groups show to see who\n" +
			"is in a group and what their user ids are.",
	}
	cmd.AddCommand(
		newGroupsMembersAddCommand(env),
		newGroupsMembersUpdateCommand(env),
		newGroupsMembersRemoveCommand(env),
	)

	return cmd
}

func newGroupsMembersAddCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <group-id> <user-id>",
		Short: "Add a member to a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID, userID, err := parseGroupMemberIDs(args)
			if err != nil {
				return err
			}

			raw, err := cmd.Flags().GetString(groupRoleFlag)
			if err != nil {
				return err
			}
			role, err := util.ParseRole(raw)
			if err != nil {
				return err
			}

			if err := env.apiv2Client.AddGroupMember(cmd.Context(), groupID, userID, role); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, map[string]any{
					"group_id": groupID,
					"user_id":  userID,
					"role":     role,
					"added":    true,
				})
			}

			_, err = fmt.Fprintf(out, "Added user %s to group %s as %s.\n", userID, groupID, role)
			return err
		},
	}

	cmd.Flags().String(groupRoleFlag, string(client.GroupRoleMember), "role for the new member: admin or member")

	return cmd
}

func newGroupsMembersUpdateCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <group-id> <user-id> --role <admin|member>",
		Short: "Update the role of a group member",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID, userID, err := parseGroupMemberIDs(args)
			if err != nil {
				return err
			}

			raw, err := cmd.Flags().GetString(groupRoleFlag)
			if err != nil {
				return err
			}
			role, err := util.ParseRole(raw)
			if err != nil {
				return err
			}

			if err := env.apiv2Client.UpdateGroupMemberRole(cmd.Context(), groupID, userID, role); err != nil {
				return err
			}

			return reportUpdate(cmd, userID, "role",
				fmt.Sprintf("User %s is now %s in group %s.", userID, role, groupID))
		},
	}

	cmd.Flags().String(groupRoleFlag, "", "new role for the member: admin or member")
	if err := cmd.MarkFlagRequired(groupRoleFlag); err != nil {
		panic(err)
	}

	return cmd
}

func newGroupsMembersRemoveCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <group-id> <user-id>",
		Aliases: []string{"rm"},
		Short:   "Remove a member from a group",
		Long: "Remove a member from a group.\n\n" +
			"The user keeps their account and can be added back at any time.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID, userID, err := parseGroupMemberIDs(args)
			if err != nil {
				return err
			}

			if err := env.apiv2Client.RemoveGroupMember(cmd.Context(), groupID, userID); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, map[string]any{
					"group_id": groupID,
					"user_id":  userID,
					"removed":  true,
				})
			}

			_, err = fmt.Fprintf(out, "Removed user %s from group %s.\n", userID, groupID)
			return err
		},
	}
}

// parseGroupMemberIDs reads the group and user a member command addresses.
func parseGroupMemberIDs(args []string) (string, string, error) {
	groupID, err := util.ParseGroupID(args[0])
	if err != nil {
		return "", "", err
	}

	userID, err := util.ParseUserID(args[1])
	if err != nil {
		return "", "", err
	}

	return groupID, userID, nil
}
