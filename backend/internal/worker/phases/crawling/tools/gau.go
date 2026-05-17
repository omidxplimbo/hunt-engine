package tools

func RunGAU(ctx Context, inputFile string) map[string]string {
	return collectFromCommand(ctx, inputFile, "gau", "gau", "--threads", "10")
}
