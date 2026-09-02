package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/util"
	"github.com/spf13/cobra"
)

const (
	accessListNameFlag        = "name"
	accessListAutoApproveFlag = "auto-approve"
	accessListYesFlag         = "yes"
)

func newAccessListsCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "access-lists",
		Aliases: []string{"access-list"},
		Short:   "Manage the access lists in your organization",
		Long: "Manage the access lists in your organization.\n\n" +
			"An access list names the models a group or API key may use. Only the models on\n" +
			"the list are allowed, whatever their kind, so a list with only chat models\n" +
			"blocks embeddings, images and audio too. Models are grouped by modality:\n" +
			"chat, embedding, image, transcription or speech.",
	}

	cmd.PersistentFlags().Bool(jsonFlag, false, "print JSON instead of a table")
	cmd.AddCommand(
		newAccessListsListCommand(env),
		newAccessListsShowCommand(env),
		newAccessListsCreateCommand(env),
		newAccessListsSetCommand(env),
		newAccessListsClearCommand(env),
		newAccessListsDeleteCommand(env),
	)

	return cmd
}

func newAccessListsListCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the access lists in your organization",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			lists, err := env.apiv2Client.AccessLists(cmd.Context())
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, lists)
			}
			if len(lists) == 0 {
				_, err := fmt.Fprintln(out, "No access lists yet.")
				return err
			}

			headers := []string{"ID", "NAME"}
			for _, modality := range client.Modalities {
				headers = append(headers, strings.ToUpper(string(modality)))
			}
			headers = append(headers, "AUTO APPROVE")

			rows := make([][]string, 0, len(lists))
			for _, list := range lists {
				row := []string{list.ID, list.Name}
				// The table shows how many models each modality allows; the
				// show command names them.
				for _, modality := range client.Modalities {
					row = append(row, strconv.Itoa(len(list.Models(modality))))
				}
				row = append(row, strconv.FormatBool(list.AutoApprove))
				rows = append(rows, row)
			}

			return writeTable(out, headers, rows)
		},
	}
}

func newAccessListsShowCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show one access list and the models it allows",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			list, err := env.apiv2Client.AccessList(cmd.Context(), id)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, list)
			}

			rows := make([][]string, 0)
			for _, modality := range client.Modalities {
				for _, model := range list.Models(modality) {
					rows = append(rows, []string{string(modality), model})
				}
			}

			if err := writeFields(out, [][2]string{
				{"ID", list.ID},
				{"Name", list.Name},
				{"Organization", list.OrganizationID},
				{"Auto approve", strconv.FormatBool(list.AutoApprove)},
				{"Models", strconv.Itoa(len(rows))},
			}); err != nil {
				return err
			}

			if len(rows) == 0 {
				return nil
			}

			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}

			return writeTable(out, []string{"MODALITY", "MODEL"}, rows)
		},
	}
}

func newAccessListsCreateCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an access list",
		Long: "Create an access list.\n\n" +
			"Pass the models to allow with one flag per modality, either repeated or\n" +
			"comma-separated. A modality left out allows no models of that kind. The list\n" +
			"is not attached to anything yet; do that in the console.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := cmd.Flags()

			name, err := flags.GetString(accessListNameFlag)
			if err != nil {
				return err
			}
			autoApprove, err := flags.GetBool(accessListAutoApproveFlag)
			if err != nil {
				return err
			}

			input := client.CreateAccessListInput{Name: name, AutoApprove: autoApprove}

			models := make(map[client.Modality][]string, len(client.Modalities))
			for _, modality := range client.Modalities {
				raw, err := flags.GetStringSlice(string(modality))
				if err != nil {
					return err
				}
				if len(raw) == 0 {
					continue
				}
				parsed, err := util.ParseModels(raw)
				if err != nil {
					return fmt.Errorf("--%s: %w", modality, err)
				}
				models[modality] = parsed
			}
			input.Chat = models[client.ModalityChat]
			input.Embedding = models[client.ModalityEmbedding]
			input.Image = models[client.ModalityImage]
			input.Transcription = models[client.ModalityTranscription]
			input.Speech = models[client.ModalitySpeech]

			created, err := env.apiv2Client.CreateAccessList(cmd.Context(), input)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, created)
			}

			_, err = fmt.Fprintf(out, "Created access list %s with id %s.\n", name, created.ID)
			return err
		},
	}

	flags := cmd.Flags()
	flags.String(accessListNameFlag, "", "name for the new access list")
	flags.Bool(accessListAutoApproveFlag, false, "approve new models matching the list automatically")
	for _, modality := range client.Modalities {
		flags.StringSlice(string(modality), nil,
			fmt.Sprintf("%s models to allow, for example %s", modality, exampleModel(modality)))
	}
	if err := cmd.MarkFlagRequired(accessListNameFlag); err != nil {
		panic(err)
	}

	return cmd
}

// exampleModel gives a plausible model id for a modality, to make the flag
// help concrete.
func exampleModel(modality client.Modality) string {
	switch modality {
	case client.ModalityEmbedding:
		return "openai/text-embedding-3-small"
	case client.ModalityImage:
		return "openai/dall-e-3"
	case client.ModalityTranscription:
		return "openai/whisper-1"
	case client.ModalitySpeech:
		return "openai/tts-1"
	default:
		return "openai/gpt-4o"
	}
}

func newAccessListsSetCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set a property on an access list",
	}
	cmd.AddCommand(
		newAccessListsSetNameCommand(env),
		newAccessListsSetAutoApproveCommand(env),
		newAccessListsSetModelsCommand(env),
	)

	return cmd
}

func newAccessListsSetNameCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "name <id> <name>",
		Short: "Rename an access list",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			name := strings.TrimSpace(args[1])
			if name == "" {
				return errors.New("missing name")
			}

			if err := env.apiv2Client.UpdateAccessListName(cmd.Context(), id, name); err != nil {
				return err
			}

			return reportUpdate(cmd, id, "name", fmt.Sprintf("Access list %s is now named %s.", id, name))
		},
	}
}

func newAccessListsSetAutoApproveCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "auto-approve <id>",
		Short: "Approve new models matching an access list automatically",
		Long: "Approve new models matching an access list automatically.\n\n" +
			"Run clear auto-approve to go back to adding models by hand.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			if err := env.apiv2Client.UpdateAccessListAutoApprove(cmd.Context(), id, true); err != nil {
				return err
			}

			return reportUpdate(cmd, id, "auto_approve",
				fmt.Sprintf("Access list %s now approves matching new models automatically.", id))
		},
	}
}

func newAccessListsSetModelsCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "models <id> <modality> <model>...",
		Short: "Set the models of one modality on an access list",
		Long: "Set the models of one modality on an access list.\n\n" +
			"The modality is chat, embedding, image, transcription or speech. Its models are\n" +
			"replaced wholesale: pass every model you want to keep. The other modalities are\n" +
			"left as they are. Use clear models to remove every model of a modality.",
		// A missing model would otherwise wipe the modality, so say what to run
		// instead rather than reporting an argument count.
		Args: func(_ *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				return errors.New("missing access list id")
			case 1:
				return errors.New("missing modality: want chat, embedding, image, transcription or speech")
			case 2:
				return errors.New("no models given: pass their ids, or run clear models to remove them all")
			default:
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			modality, err := util.ParseModality(args[1])
			if err != nil {
				return err
			}

			models, err := util.ParseModels(args[2:])
			if err != nil {
				return err
			}

			err = env.apiv2Client.UpdateAccessListModels(cmd.Context(), id, map[client.Modality][]string{modality: models})
			if err != nil {
				return err
			}

			return reportUpdate(cmd, id, string(modality),
				fmt.Sprintf("Access list %s now allows %d %s %s: %s.",
					id, len(models), modality, pluralModels(len(models)), strings.Join(models, ", ")))
		},
	}
}

func newAccessListsClearCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear a property on an access list",
	}
	cmd.AddCommand(
		newAccessListsClearAutoApproveCommand(env),
		newAccessListsClearModelsCommand(env),
	)

	return cmd
}

func newAccessListsClearAutoApproveCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "auto-approve <id>",
		Short: "Stop approving new models on an access list automatically",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			if err := env.apiv2Client.UpdateAccessListAutoApprove(cmd.Context(), id, false); err != nil {
				return err
			}

			return reportUpdate(cmd, id, "auto_approve",
				fmt.Sprintf("Access list %s no longer approves new models automatically.", id))
		},
	}
}

func newAccessListsClearModelsCommand(env environment) *cobra.Command {
	return &cobra.Command{
		Use:   "models <id> <modality>...",
		Short: "Remove every model of one or more modalities from an access list",
		Long: "Remove every model of one or more modalities from an access list.\n\n" +
			"Anything attached to the list can then use no models of those kinds at all,\n" +
			"since only the models on the list are allowed.",
		Args: func(_ *cobra.Command, args []string) error {
			switch len(args) {
			case 0:
				return errors.New("missing access list id")
			case 1:
				return errors.New("missing modality: want chat, embedding, image, transcription or speech")
			default:
				return nil
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			cleared := make(map[client.Modality][]string, len(args)-1)
			names := make([]string, 0, len(args)-1)
			for _, raw := range args[1:] {
				modality, err := util.ParseModality(raw)
				if err != nil {
					return err
				}
				if _, seen := cleared[modality]; seen {
					continue
				}
				cleared[modality] = nil
				names = append(names, string(modality))
			}

			if err := env.apiv2Client.UpdateAccessListModels(cmd.Context(), id, cleared); err != nil {
				return err
			}

			return reportUpdate(cmd, id, strings.Join(names, ","),
				fmt.Sprintf("Removed every %s model from access list %s.", strings.Join(names, " and "), id))
		},
	}
}

func newAccessListsDeleteCommand(env environment) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete an access list",
		Long: "Delete an access list.\n\n" +
			"Deletion is permanent. It is refused while the list is still attached to a\n" +
			"group or API key, so detach it everywhere first.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := util.ParseAccessListID(args[0])
			if err != nil {
				return err
			}

			skipPrompt, err := cmd.Flags().GetBool(accessListYesFlag)
			if err != nil {
				return err
			}

			if !skipPrompt {
				confirmed, err := util.Confirm(
					cmd.InOrStdin(),
					cmd.OutOrStdout(),
					fmt.Sprintf("Delete access list %s? This cannot be undone [y/N]: ", id),
				)
				if err != nil {
					return err
				}
				if !confirmed {
					_, err := fmt.Fprintln(cmd.OutOrStdout(), "Left it alone.")
					return err
				}
			}

			if err := env.apiv2Client.DeleteAccessList(cmd.Context(), id); err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if jsonOutput(cmd) {
				return printJSON(out, map[string]any{"id": id, "deleted": true})
			}

			_, err = fmt.Fprintf(out, "Deleted access list %s.\n", id)
			return err
		},
	}

	cmd.Flags().BoolP(accessListYesFlag, "y", false, "delete without asking first")

	return cmd
}

// pluralModels picks the noun for a count of models.
func pluralModels(count int) string {
	if count == 1 {
		return "model"
	}

	return "models"
}
