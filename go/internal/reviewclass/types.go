package reviewclass

type ID string

const (
	C0  ID = "C0"
	C1  ID = "C1"
	C2  ID = "C2"
	C3  ID = "C3"
	C4  ID = "C4"
	C5  ID = "C5"
	C6  ID = "C6"
	C7  ID = "C7"
	C8  ID = "C8"
	C9  ID = "C9"
	C10 ID = "C10"
	C11 ID = "C11"
	C12 ID = "C12"
	C13 ID = "C13"
)

type Seat struct {
	Any         []string `json:"any"`
	Lens        string   `json:"lens,omitempty"`
	Tier        string   `json:"tier,omitempty"`
	OtherFamily bool     `json:"other_family,omitempty"`
	Planned     string   `json:"planned,omitempty"`
}

type Check struct {
	ID      string `json:"id"`
	Rule    string `json:"rule"`
	Planned string `json:"planned,omitempty"`
}

type Class struct {
	ID           ID       `json:"id"`
	Name         string   `json:"name"`
	Detect       string   `json:"detect"`
	Seats        []Seat   `json:"seats"`
	SecurityWhen []string `json:"security_when,omitempty"`
	Checks       []Check  `json:"checks"`
	Cap          string   `json:"cap"`
	match        func(Surface, string) bool
}

type Surface struct {
	Protected  func(path string) bool
	Guarantees func(path string) []string
}

type Diff struct {
	Paths       []string `json:"paths"`
	CommentOnly []string `json:"comment_only"`
	StageFlips  []string `json:"stage_flips"`
}

type Hit struct {
	Class ID       `json:"class"`
	Name  string   `json:"name"`
	Paths []string `json:"paths"`
}

type Requirements struct {
	Classes []Hit   `json:"classes"`
	Seats   []Seat  `json:"seats"`
	Checks  []Check `json:"checks"`
}
