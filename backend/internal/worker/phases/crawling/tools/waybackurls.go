package tools

func RunWaybackURLs(ctx Context, inputFile string) map[string]string {
	return collectFromCommand(ctx, inputFile, "wayback", "waybackurls")
}
