package panestream

var (
	_ *LivenessCenter = NewLivenessCenter()
	_ bool            = NewLivenessCenter().Busy("")
	_ bool            = NewLivenessCenter().Changed("")
	_ bool            = NewLivenessCenter().BusyOf("", PaneProfile{})
	_ bool            = (*LivenessCenter)(nil).BusyOf("", PaneProfile{})
)
