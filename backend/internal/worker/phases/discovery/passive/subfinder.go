package passive

import "log"

func RunSubfinder(ctx Context, providerConfigPath string) []string {
	args := []string{"-d", ctx.Domain, "-silent", "-all"}
	if providerConfigPath != "" {
		args = append(args, "-pc", providerConfigPath)
	}

	output, err := ctx.RunCommand("subfinder", args...)
	if err != nil {
		log.Printf("❌ subfinder error/killed: %v\n", err)
		return []string{}
	}

	return normalizeLines(output, ctx.Domain)
}
