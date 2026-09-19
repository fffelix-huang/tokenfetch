// Package completion holds tokenfetch's shell completion scripts, printed by
// --bash, --zsh and --fish. Keep them in sync with usageText in cmd/tokenfetch.
package completion

import _ "embed"

var (
	//go:embed completion.bash
	Bash string
	//go:embed completion.zsh
	Zsh string
	//go:embed completion.fish
	Fish string
)
