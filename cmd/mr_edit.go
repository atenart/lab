package cmd

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/rsteube/carapace"
	"github.com/spf13/cobra"
	gitlab "github.com/xanzy/go-gitlab"
	"github.com/zaquestion/lab/internal/action"
	lab "github.com/zaquestion/lab/internal/gitlab"
)

var mrEditCmd = &cobra.Command{
	Use:     "edit [remote] <id>[:<comment_id>]",
	Aliases: []string{"update"},
	Short:   "Edit or update an MR",
	Long:    ``,
	Example: `lab MR edit <id>                                # update MR via $EDITOR
lab MR update <id>                              # same as above
lab MR update <branch-name>                     # same, but get MR ID from local branch
lab MR edit <id> -m "new title"                 # update title
lab MR edit <id> -m "new title" -m "new desc"   # update title & description
lab MR edit <id> -l newlabel --unlabel oldlabel # relabel MR
lab MR edit <id>:<comment_id>                   # update a comment on MR`,
	PersistentPreRun: LabPersistentPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		commentNum, branchArgs, err := filterCommentArg(args)
		if err != nil {
			log.Fatal(err)
		}

		rn, id, err := parseArgsWithGitBranchMR(branchArgs)
		if err != nil {
			log.Fatal(err)
		}
		mrNum := int(id)

		if mrNum == 0 {
			fmt.Println("Error: Cannot determine MR id.")
			os.Exit(1)
		}

		mr, err := lab.MRGet(rn, mrNum)
		if err != nil {
			log.Fatal(err)
		}

		linebreak, err := cmd.Flags().GetBool("force-linebreak")
		if err != nil {
			log.Fatal(err)
		}

		// Edit a comment on the MR
		if commentNum != 0 {
			replyNote(rn, true, mrNum, commentNum, true, true, "", linebreak, false, nil)
			return
		}

		// get the labels to add
		labelTerms, err := cmd.Flags().GetStringSlice("label")
		if err != nil {
			log.Fatal(err)
		}
		labels, err := MapLabels(rn, labelTerms)
		if err != nil {
			log.Fatal(err)
		}

		// get the labels to remove
		unlabelTerms, err := cmd.Flags().GetStringSlice("unlabel")
		if err != nil {
			log.Fatal(err)
		}
		unlabels, err := MapLabels(rn, unlabelTerms)
		if err != nil {
			log.Fatal(err)
		}

		labels, labelsChanged, err := editGetLabels(mr.Labels, labels, unlabels)
		if err != nil {
			log.Fatal(err)
		}

		// get the assignees to add
		assignees, err := cmd.Flags().GetStringSlice("assign")
		if err != nil {
			log.Fatal(err)
		}

		// get the assignees to remove
		unassignees, err := cmd.Flags().GetStringSlice("unassign")
		if err != nil {
			log.Fatal(err)
		}

		draft, err := cmd.Flags().GetBool("draft")
		if err != nil {
			log.Fatal(err)
		}

		ready, err := cmd.Flags().GetBool("ready")
		if err != nil {
			log.Fatal(err)
		}

		if draft && ready {
			log.Fatal("--draft and --ready cannot be used together")
		}

		currentAssignees := mrGetCurrentAssignees(mr)
		assigneeIDs, assigneesChanged, err := getUpdateAssignees(currentAssignees, assignees, unassignees)
		if err != nil {
			log.Fatal(err)
		}

		milestoneName, err := cmd.Flags().GetString("milestone")
		if err != nil {
			log.Fatal(err)
		}
		updateMilestone := cmd.Flags().Lookup("milestone").Changed
		milestoneID := -1

		if milestoneName != "" {
			ms, err := lab.MilestoneGet(rn, milestoneName)
			if err != nil {
				log.Fatal(err)
			}
			milestoneID = ms.ID
		}

		// get all of the "message" flags
		msgs, err := cmd.Flags().GetStringSlice("message")
		if err != nil {
			log.Fatal(err)
		}
		title, body, err := editGetTitleDescription(mr.Title, mr.Description, msgs, cmd.Flags().NFlag())
		if err != nil {
			_, f, l, _ := runtime.Caller(0)
			log.Fatal(f+":"+strconv.Itoa(l)+" ", err)
		}
		if title == "" {
			log.Fatal("aborting: empty mr title")
		}

		isWIP := strings.EqualFold(title[0:4], "wip:")
		isDraft := strings.EqualFold(title[0:6], "draft:") ||
			strings.EqualFold(title[0:7], "[draft]") ||
			strings.EqualFold(title[0:7], "(draft)")

		if ready {
			if isWIP {
				title = strings.TrimPrefix(title, title[0:4])
			} else if isDraft {
				if title[0] == '(' || title[0] == '[' {
					title = strings.TrimPrefix(title, title[0:7])
				} else {
					title = strings.TrimPrefix(title, title[0:6])
				}
			}
		}

		if draft {
			if !isWIP && !isDraft {
				title = "Draft: " + title
			}
		}

		abortUpdate := title == mr.Title && body == mr.Description && !labelsChanged && !assigneesChanged && !updateMilestone
		if abortUpdate {
			log.Fatal("aborting: no changes")
		}

		if linebreak {
			body = textToMarkdown(body)
		}

		opts := &gitlab.UpdateMergeRequestOptions{
			Title:       &title,
			Description: &body,
		}

		if labelsChanged {
			opts.Labels = labels
		}

		if assigneesChanged {
			opts.AssigneeIDs = assigneeIDs
		}

		if updateMilestone {
			opts.MilestoneID = &milestoneID
		}

		mrURL, err := lab.MRUpdate(rn, int(mrNum), opts)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(mrURL)
	},
}

// mrGetCurrentAssignees returns a string slice of the current assignees'
// usernames
func mrGetCurrentAssignees(mr *gitlab.MergeRequest) []string {
	currentAssignees := make([]string, len(mr.Assignees))
	if len(mr.Assignees) > 0 && mr.Assignees[0].Username != "" {
		for i, a := range mr.Assignees {
			currentAssignees[i] = a.Username
		}
	}
	return currentAssignees
}

func init() {
	mrEditCmd.Flags().StringSliceP("message", "m", []string{}, "use the given <msg>; multiple -m are concatenated as separate paragraphs")
	mrEditCmd.Flags().StringSliceP("label", "l", []string{}, "add the given label(s) to the merge request")
	mrEditCmd.Flags().StringSliceP("unlabel", "", []string{}, "remove the given label(s) from the merge request")
	mrEditCmd.Flags().StringSliceP("assign", "a", []string{}, "add an assignee by username")
	mrEditCmd.Flags().StringSliceP("unassign", "", []string{}, "remove an assignee by username")
	mrEditCmd.Flags().String("milestone", "", "set milestone")
	mrEditCmd.Flags().Bool("force-linebreak", false, "append 2 spaces to the end of each line to force markdown linebreaks")
	mrEditCmd.Flags().Bool("draft", false, "mark the merge request as draft")
	mrEditCmd.Flags().Bool("ready", false, "mark the merge request as ready")
	mrEditCmd.Flags().SortFlags = false

	mrCmd.AddCommand(mrEditCmd)
	carapace.Gen(mrEditCmd).PositionalCompletion(
		action.Remotes(),
		action.MergeRequests(mrList),
	)
}
