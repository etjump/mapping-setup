// Command etmapping-setup sets up a complete ET/ETJump mapping environment.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/etjump/mapping-setup/internal/fetch"
	"github.com/etjump/mapping-setup/internal/install"
	"github.com/etjump/mapping-setup/internal/platform"
	"github.com/etjump/mapping-setup/internal/sources"
	"github.com/etjump/mapping-setup/internal/ui"
)

// version is set at build time by the Makefile.
var version = "dev"

const logName = "etmapping-setup.log"

func main() {
	code := run()
	ui.WaitForEnter()
	os.Exit(code)
}

func run() int {
	ui.Init()
	con := &ui.Console{}
	defer con.Close()

	p := platform.Detect()
	con.Heading("ETJump Mapping Setup %s (%s)", version, p)
	fmt.Println()

	if err := p.CheckSupported(); err != nil {
		con.Error("%v", err)
		return 1
	}
	if err := install.CheckNotRoot(p); err != nil {
		con.Error("%v", err)
		return 1
	}
	if !ui.Interactive() {
		con.Error("please run the installer in a terminal window")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := fetch.New(version)

	con.Linef("Checking the internet connection...")
	if failed := install.CheckConnectivity(ctx, client); len(failed) > 0 {
		con.Error("cannot reach %s. Check your internet connection or firewall and try again.", strings.Join(failed, ", "))
		return 1
	}
	fmt.Println()

	choices, err := ask(p)
	if errors.Is(err, ui.ErrAborted) {
		con.Linef("\nCancelled. Nothing was installed.")
		return 1
	}
	if err != nil {
		con.Error("%v", err)
		return 1
	}

	fmt.Println()
	con.Linef("Looking up the latest versions...")
	plan, err := install.Resolve(ctx, client, choices, p, func(name string) {
		con.Info("%s", name)
	})
	if err != nil {
		con.Error("%v", err)
		return 1
	}

	printSummary(con, plan)
	size, _ := plan.DownloadSize()
	if err := install.CheckDiskSpace(choices.Root, size); err != nil {
		con.Error("%v", err)
		return 1
	}

	ok, err := ui.Confirm("Install now?", true)
	if err != nil || !ok {
		con.Linef("\nCancelled. Nothing was installed.")
		return 1
	}

	if err := os.MkdirAll(choices.Root, 0o755); err != nil {
		con.Error("creating %s: %v", choices.Root, err)
		return 1
	}
	logPath := filepath.Join(choices.Root, logName)
	if err := con.OpenLog(logPath); err != nil {
		con.Error("creating the log file: %v", err)
		return 1
	}
	con.Log("setup %s on %s, choices: %+v", version, p, choices)

	if err := install.Run(ctx, client, plan, version, con); err != nil {
		con.ProgressDone()
		if ctx.Err() != nil {
			con.Error("the install was interrupted")
		} else {
			con.Error("%v", err)
		}
		con.Linef("\nRun the installer again and pick the same folder to resume; finished downloads are kept.")
		con.Linef("Details are in %s", logPath)
		return 1
	}

	printFinished(con, plan, logPath)
	offerLaunch(con, plan)
	return 0
}

// ask collects all answers before anything is installed.
func ask(p platform.Platform) (install.Choices, error) {
	var ch install.Choices

	for {
		var check *install.RootCheck
		_, err := ui.InputFolder("Install folder", install.DefaultRoot(), func(s string) (string, error) {
			c, err := install.CheckRoot(s, p)
			if err != nil {
				return "", err
			}
			check = c
			return c.Path, nil
		})
		if err != nil {
			return ch, err
		}

		if m := check.Existing; m != nil {
			question := "This folder already has a mapping setup. Update everything to the latest versions, keeping your previous choices?"
			if !m.Complete() {
				question = "A previous install in this folder did not finish. Continue it with the same choices?"
			}
			yes, err := ui.Confirm(question, true)
			if err != nil {
				return ch, err
			}
			if yes {
				ch = m.Choices
				ch.Root = check.Path
				return ch, nil
			}
			fmt.Println("Choose another folder then.")
			continue
		}
		ch.Root = check.Path
		break
	}

	engine, err := ui.Select("Game engine and architecture", items(p, platform.Engines))
	if err != nil {
		return ch, err
	}
	ch.Engine = platform.Engines[engine].ID

	editorOptions := []platform.Option{platform.InstallRadiant, platform.ExistingEditor}
	editor, err := ui.Select("NetRadiant-custom", items(p, editorOptions))
	if err != nil {
		return ch, err
	}
	ch.InstallRadiant = editorOptions[editor].ID == platform.InstallRadiant.ID
	if !ch.InstallRadiant {
		ch.EditorDir, err = ui.InputFolder("Your level editor's folder", "", install.CheckEditor)
		if err != nil {
			return ch, err
		}
	}

	picked, err := ui.MultiSelect("Extras", items(p, platform.Extras))
	if err != nil {
		return ch, err
	}
	ch.Extras = []string{}
	for i, yes := range picked {
		if yes {
			ch.Extras = append(ch.Extras, platform.Extras[i].ID)
		}
	}
	return ch, nil
}

func items(p platform.Platform, opts []platform.Option) []ui.Item {
	var list []ui.Item
	for _, o := range opts {
		ok, reason := o.AvailableOn(p)
		list = append(list, ui.Item{Label: o.Label, Disabled: !ok, Note: reason})
	}
	return list
}

func printSummary(con *ui.Console, plan *install.Plan) {
	ch := plan.Choices
	fmt.Println()
	con.Heading("Summary")
	con.Linef("  Install folder:  %s", ch.Root)
	if ch.InstallRadiant {
		con.Linef("  Level editor:    NetRadiant-custom, installed into %s", sources.RadiantDir(ch.Root))
	} else {
		con.Linef("  Level editor:    your existing editor in %s", ch.EditorDir)
	}
	fmt.Println()

	for _, comp := range plan.Components {
		var size int64
		for _, f := range comp.Files {
			size += f.Size
		}
		sizeText := "size unknown"
		if size > 0 {
			sizeText = fetch.FormatSize(size)
		}
		con.Linef("  %-26s %-16s %s", comp.Name, comp.Version, sizeText)
	}

	var unavailable []string
	for _, o := range append([]platform.Option{platform.InstallRadiant}, platform.Extras...) {
		if ok, reason := o.AvailableOn(plan.Target.Platform); !ok {
			unavailable = append(unavailable, fmt.Sprintf("%s (%s)", o.Label, reason))
		}
	}
	if len(unavailable) > 0 {
		fmt.Println()
		con.Linef("  Skipped: %s", strings.Join(unavailable, ", "))
	}

	total, complete := plan.DownloadSize()
	about := "about "
	if !complete {
		about = "at least "
	}
	fmt.Println()
	con.Linef("  Total download: %s%s", about, fetch.FormatSize(total))
	fmt.Println()
}

func printFinished(con *ui.Console, plan *install.Plan, logPath string) {
	ch, t := plan.Choices, plan.Target
	fmt.Println()
	con.Heading("Done! Your mapping setup is ready.")
	fmt.Println()
	con.Linef("  Folder:        %s", ch.Root)
	con.Linef("  Game:          %s", filepath.Join(ch.Root, sources.GameExecutable(t)))
	con.Linef("                 start it with  +set fs_game etjump  to play and test with ETJump")
	if ch.InstallRadiant {
		if exe := sources.RadiantExecutable(sources.RadiantDir(ch.Root)); exe != "" {
			con.Linef("  Level editor:  %s", exe)
		}
	} else {
		con.Linef("  Level editor:  %s", ch.EditorDir)
		con.Linef("                 set Edit > Preferences > Paths > Engine Path to %s", install.RadiantEnginePath(ch.Root))
	}
	for _, id := range ch.Extras {
		con.Linef("  %-14s %s", id+":", filepath.Join(ch.Root, "tools", id))
	}

	notes := install.LinuxHints(ch, t.Platform, t.Is32Bit())
	notes = append(notes, "Two ETJump entities (alarm_box, props_chair_chat) use models that only exist in RTCW and cannot be included.")
	fmt.Println()
	con.Heading("Notes")
	for _, n := range notes {
		con.Linef("  - %s", n)
	}
	fmt.Println()
	con.Linef("A full log is in %s", logPath)
}

func offerLaunch(con *ui.Console, plan *install.Plan) {
	if !plan.Choices.InstallRadiant {
		return
	}
	dir := sources.RadiantDir(plan.Choices.Root)
	exe := sources.RadiantExecutable(dir)
	if exe == "" {
		return
	}
	fmt.Println()
	yes, err := ui.Confirm("Open NetRadiant-custom now?", true)
	if err != nil || !yes {
		return
	}
	cmd := exec.Command(exe)
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		con.Error("could not start NetRadiant-custom: %v", err)
		return
	}
	cmd.Process.Release()
}
