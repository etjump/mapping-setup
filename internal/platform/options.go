package platform

// Option is a user-selectable choice that is only available on some
// platforms. Unavailable options are still shown, greyed out.
type Option struct {
	ID        string
	Label     string
	Platforms []OS
}

var (
	desktop    = []OS{Windows, Linux}
	everywhere = []OS{Windows, Linux, MacOS}
)

// Engines are the game engine choices, architecture included.
var Engines = []Option{
	{ID: "etlegacy-64", Label: "ET: Legacy (64-bit)", Platforms: everywhere},
	{ID: "etlegacy-32", Label: "ET: Legacy (32-bit)", Platforms: desktop},
	{ID: "ete-64", Label: "ETe (64-bit)", Platforms: desktop},
	{ID: "ete-32", Label: "ETe (32-bit)", Platforms: desktop},
	{ID: "et260b", Label: "ET 2.60b (32-bit)", Platforms: []OS{Windows}},
}

// Editor choices. Using an existing editor works everywhere.
var (
	InstallRadiant = Option{ID: "netradiant-custom", Label: "Install NetRadiant-custom", Platforms: desktop}
	ExistingEditor = Option{ID: "existing-editor", Label: "I already have a level editor", Platforms: everywhere}
)

// Extras are the optional tools.
var Extras = []Option{
	{ID: "pack3r", Label: "Pack3r", Platforms: desktop},
	{ID: "qtpack3r", Label: "QtPack3r", Platforms: desktop},
}

// AvailableOn reports whether the option can be picked on p, and if not,
// the reason to show next to it.
func (o Option) AvailableOn(p Platform) (bool, string) {
	for _, supported := range o.Platforms {
		if supported == p.OS {
			return true, ""
		}
	}
	return false, "not available on " + p.Name()
}
