package theme

// Icons holds every glyph the UI draws. Nerd Font glyphs come from the
// Font Awesome range (U+F000–U+F2E0), which has been stable across Nerd
// Font releases.
type Icons struct {
	Inbox, Today, Upcoming, Overdue, Completed string
	Project, Tag, Due, Subtasks, Note          string
	Todo, InProgress, Done                     string
	Priority                                   string // left-edge priority bar
	Selected                                   string // sidebar cursor
	Expanded, Collapsed                        string
	Branch, BranchLast                         string
	Check, Cross, Warn, Info                   string
}

// NerdIcons requires a Nerd Font.
var NerdIcons = Icons{
	Inbox: "", Today: "", Upcoming: "", Overdue: "", Completed: "",
	Project: "", Tag: "", Due: "", Subtasks: "", Note: "",
	Todo: "", InProgress: "", Done: "",
	Priority: "▎", Selected: "▌",
	Expanded: "", Collapsed: "",
	Branch: "├", BranchLast: "└",
	Check: "", Cross: "", Warn: "", Info: "",
}

// ASCIIIcons works in any terminal font.
var ASCIIIcons = Icons{
	Inbox: "»", Today: "*", Upcoming: ">", Overdue: "!", Completed: "v",
	Project: "@", Tag: "#", Due: "~", Subtasks: "+", Note: "=",
	Todo: "[ ]", InProgress: "[~]", Done: "[x]",
	Priority: "|", Selected: ">",
	Expanded: "v", Collapsed: ">",
	Branch: "|-", BranchLast: "`-",
	Check: "ok", Cross: "x", Warn: "!", Info: "i",
}

// IconSet returns the icons for a config value ("nerd" or "ascii").
func IconSet(name string) Icons {
	if name == "ascii" {
		return ASCIIIcons
	}
	return NerdIcons
}
