package commitgate

const (
	reviewClassCommentOnly = "comment-only"
	reviewClassDocs        = "docs"
	reviewClassCode        = "code"
)

type reviewOutcome struct {
	waiver string
	class  string
}

func reviewClassOf(files []string) string {
	if isDocsOnly(files) {
		return reviewClassDocs
	}
	return reviewClassCode
}
