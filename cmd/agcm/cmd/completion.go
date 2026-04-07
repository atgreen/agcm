// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion script",
	Long: `Generate a shell completion script for agcm.

To load completions:

  bash:
    $ source <(agcm completion bash)

    # To load completions for each session, execute once:
    # Linux:
    $ agcm completion bash > /etc/bash_completion.d/agcm
    # macOS:
    $ agcm completion bash > $(brew --prefix)/etc/bash_completion.d/agcm

  zsh:
    # If shell completion is not already enabled in your environment,
    # you will need to enable it. You can execute the following once:
    $ echo "autoload -U compinit; compinit" >> ~/.zshrc

    # To load completions for each session, execute once:
    $ agcm completion zsh > "${fpath[1]}/_agcm"

    # You will need to start a new shell for this setup to take effect.

  fish:
    $ agcm completion fish | source

    # To load completions for each session, execute once:
    $ agcm completion fish > ~/.config/fish/completions/agcm.fish

  powershell:
    PS> agcm completion powershell | Out-String | Invoke-Expression

    # To load completions for every new session, add the output to your profile:
    PS> agcm completion powershell > agcm.ps1
    # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletionV2(os.Stdout, true)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
