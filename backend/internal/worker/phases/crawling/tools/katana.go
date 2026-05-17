package tools

func RunKatana(ctx Context, inputFile string) map[string]string {
	return collectFromCommand(ctx, inputFile, "katana", "katana", "-list", inputFile, "-jc", "-kf", "-silent", "-c", "10")
}
