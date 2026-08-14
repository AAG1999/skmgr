// Copyright 2026 AAG1999
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:       "completion [bash|zsh|fish|powershell]",
	Short:     "Generate a shell completion script",
	ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
	Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Long: `Generate a completion script for your shell.

Bash:
  # Once, for the current shell
  source <(skmgr completion bash)

  # Permanently
  skmgr completion bash > /etc/bash_completion.d/skmgr        # Linux
  skmgr completion bash > $(brew --prefix)/etc/bash_completion.d/skmgr  # macOS

Zsh:
  # If completion is not already enabled, run this once
  echo "autoload -U compinit; compinit" >> ~/.zshrc

  skmgr completion zsh > "${fpath[1]}/_skmgr"

Fish:
  skmgr completion fish > ~/.config/fish/completions/skmgr.fish

PowerShell:
  skmgr completion powershell | Out-String | Invoke-Expression`,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()

		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletionV2(out, true)
		case "zsh":
			return cmd.Root().GenZshCompletion(out)
		case "fish":
			return cmd.Root().GenFishCompletion(out, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(out)
		default:
			return fmt.Errorf("unsupported shell %q", args[0])
		}
	},
}

func init() {
	// Cobra ships its own completion command, but it is hidden behind a
	// generated help topic and does not document how to install the output.
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.AddCommand(completionCmd)
}
