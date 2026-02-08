package main

import (
    "fmt"
    "os"
    
    "github.com/spf13/cobra"
)

func debugCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "debug",
        Short: "Debug config loading",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Println("ANTHROPIC_API_KEY from env:")
            key := os.Getenv("ANTHROPIC_API_KEY")
            if key == "" {
                fmt.Println("  (empty)")
            } else {
                fmt.Printf("  %s...%s (len=%d)\n", key[:10], key[len(key)-5:], len(key))
            }
        },
    }
}
