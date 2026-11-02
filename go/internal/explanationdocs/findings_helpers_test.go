package explanationdocs

func changedAreaFailures(section string, changed, material []string) []string {
	failures, _ := changedAreaFindings(section, changed, material)
	return failures
}

func validateDocument(body string, cycle int, baseSHA string, changed, material []string) []string {
	failures, _ := documentFindings(body, cycle, baseSHA, changed, material)
	return failures
}
