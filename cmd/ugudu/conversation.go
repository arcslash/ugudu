package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func conversationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "conversation",
		Aliases: []string{"conv"},
		Short:   "Manage team conversations",
		Long: `View and manage team conversation history.

Conversations are automatically persisted and restored when the daemon restarts.
This means your chat history with teams survives crashes and restarts.`,
	}

	cmd.AddCommand(conversationListCmd())
	cmd.AddCommand(conversationShowCmd())
	cmd.AddCommand(conversationClearCmd())

	return cmd
}

func conversationListCmd() *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "list [team-name]",
		Short: "List conversations for a team",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			conversations, err := client.ListConversations(ctx, args[0], limit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if len(conversations) == 0 {
				fmt.Println("No conversations found.")
				return
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTARTED\tLAST MESSAGE\tSTATUS")
			fmt.Fprintln(w, "──\t───────\t────────────\t──────")

			for _, conv := range conversations {
				id, _ := conv["id"].(string)
				startedAt, _ := conv["started_at"].(string)
				lastMsg, _ := conv["last_message_at"].(string)
				status, _ := conv["status"].(string)

				// Truncate ID for display
				if len(id) > 20 {
					id = id[:17] + "..."
				}

				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", id, startedAt, lastMsg, status)
			}
			w.Flush()
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "number of conversations to show")

	return cmd
}

func conversationShowCmd() *cobra.Command {
	var outputJSON bool

	cmd := &cobra.Command{
		Use:   "show [conversation-id]",
		Short: "Show conversation history",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			history, err := client.GetConversationHistory(ctx, args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			if outputJSON {
				data, _ := json.MarshalIndent(history, "", "  ")
				fmt.Println(string(data))
				return
			}

			if len(history) == 0 {
				fmt.Println("No messages in this conversation.")
				return
			}

			for _, msg := range history {
				memberID, _ := msg["member_id"].(string)
				role, _ := msg["role"].(string)
				content, _ := msg["content"].(string)

				// Color-code by role
				switch role {
				case "user":
					fmt.Printf("\n[USER → %s]\n%s\n", memberID, content)
				case "assistant":
					fmt.Printf("\n[%s]\n%s\n", memberID, content)
				case "system":
					fmt.Printf("\n[SYSTEM]\n%s\n", content)
				}
			}
		},
	}

	cmd.Flags().BoolVar(&outputJSON, "json", false, "output as JSON")

	return cmd
}

func conversationClearCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear [team-name]",
		Short: "Clear conversation history for a team",
		Long: `Clear all conversation history for a team.

This starts a fresh conversation context. Use this when you want the team
to start without memory of previous interactions.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if !force {
				fmt.Printf("This will clear all conversation history for team '%s'.\n", args[0])
				fmt.Print("Are you sure? [y/N] ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					fmt.Println("Aborted.")
					return
				}
			}

			client, err := requireDaemon()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if err := client.ClearConversation(ctx, args[0]); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Conversation history cleared for team '%s'.\n", args[0])
			fmt.Println("A new conversation will start on the next message.")
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation")

	return cmd
}
