package covchanged

import "fmt"

type Verdict struct {
	Changed int         `json:"changed"`
	Full    int         `json:"full"`
	Refused []Func      `json:"refused"`
	Exempt  []Exemption `json:"exempt"`
}

func Judge(r Report) Verdict {
	v := Verdict{Changed: len(r.Funcs), Refused: []Func{}, Exempt: append([]Exemption{}, r.Exempt...)}
	for _, f := range r.Funcs {
		if f.Full() {
			v.Full++
			continue
		}
		v.Refused = append(v.Refused, f)
	}
	return v
}

func (v Verdict) Clean() bool { return len(v.Refused) == 0 }

func (f Func) String() string {
	if !f.Profiled {
		return fmt.Sprintf("%s:%d %s no profile", f.File, f.Line, f.Key)
	}
	return fmt.Sprintf("%s:%d %s %.1f%% (%d/%d)", f.File, f.Line, f.Key, f.Percent(), f.Covered, f.Statements)
}

func (v Verdict) Problems() []string {
	var out []string
	for _, f := range v.Refused {
		out = append(out, "REFUSED "+f.String())
	}
	return out
}
