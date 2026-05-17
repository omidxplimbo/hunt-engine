package passive

import "log"

func RunAssetfinder(ctx Context) []string {
	output, err := ctx.RunCommand("assetfinder", "--subs-only", ctx.Domain)
	if err != nil {
		log.Printf("❌ assetfinder error/killed: %v\n", err)
		return []string{}
	}

	return normalizeLines(output, ctx.Domain)
}
