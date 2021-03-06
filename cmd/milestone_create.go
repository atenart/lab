package cmd

import (
	"log"

	"github.com/rsteube/carapace"
	"github.com/spf13/cobra"
	gitlab "github.com/xanzy/go-gitlab"
	"github.com/zaquestion/lab/internal/action"
	lab "github.com/zaquestion/lab/internal/gitlab"
)

var milestoneCreateCmd = &cobra.Command{
	Use:              "create [remote] <name>",
	Aliases:          []string{"add"},
	Short:            "Create a new milestone",
	Long:             ``,
	Example:          "lab milestone create my-milestone",
	PersistentPreRun: LabPersistentPreRun,
	Args:             cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		rn, title, err := parseArgsRemoteAndProject(args)
		if err != nil {
			log.Fatal(err)
		}

		desc, err := cmd.Flags().GetString("description")
		if err != nil {
			log.Fatal(err)
		}

		err = lab.MilestoneCreate(rn, &gitlab.CreateMilestoneOptions{
			Title:       &title,
			Description: &desc,
		})

		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	milestoneCreateCmd.Flags().String("description", "", "description of the new milestone")
	milestoneCmd.AddCommand(milestoneCreateCmd)
	carapace.Gen(milestoneCmd).PositionalCompletion(
		action.Remotes(),
	)
}
