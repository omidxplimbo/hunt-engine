package passive

import "log"

func RunCero(ctx Context) []string {
	output, err := ctx.RunCommand("cero", "-d", ctx.Domain)
	if err != nil {
		log.Printf("⚠️ CERO error/killed: %v\n", err)
		return []string{}
	}

	results := normalizeLines(output, ctx.Domain)
	if len(results) > 0 {
		log.Printf(" CERO scraped %d domains from SSL certificates\n", len(results))
	}

	return results
}
