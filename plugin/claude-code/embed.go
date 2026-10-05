package claudecode

import "embed"

// Skills embeds the Claude skills tree.
//
//go:embed skills
var Skills embed.FS

// FS is an alias for Skills.
var FS = Skills

// Dispatch embeds the lucind dispatch markdown block.
//
//go:embed claudemd/lucind-dispatch.md
var Dispatch []byte
