package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dagger/dagger/internal/tui"
	"github.com/spf13/cobra"
	"github.com/vito/progrock"
)

func main() {
	replayCmd := &cobra.Command{
		Use:          "replay [journal]",
		Long:         "Replay a journal file",
		Short:        "Replay a journal file",
		Args:         cobra.MinimumNArgs(1),
		SilenceUsage: true,
		RunE:         Replay,
	}

	if err := replayCmd.Execute(); err != nil {
		panic(err)
	}
}

func Replay(cmd *cobra.Command, args []string) error {
	if os.Getenv("_EXPERIMENTAL_DAGGER_INTERACTIVE_TUI") != "" {
		return replayInteractiveTUI(cmd.Context(), args[0])
	} else {
		return replayInternalTUI(cmd.Context(), args[0])
	}
}

func replayInternalTUI(ctx context.Context, journal string) error {
	tape := progrock.NewTape()
	tape.ShowAllOutput(true)
	tape.ShowInternal(true)

	tape.MessageLevel(progrock.MessageLevel_DEBUG)
	progrock.DefaultUI().Run(ctx, tape, func(ctx context.Context, ui progrock.UIClient) error {
		recorder := progrock.NewRecorder(tape)
		iterateOverEvents(journal, func(event *progrock.StatusUpdate) error {
			return recorder.Record(event)
		})

		recorder.Close()
		recorder.Complete()

		return nil
	})
	return nil
}

func replayInteractiveTUI(ctx context.Context, journal string) error {
	progR, progW := progrock.Pipe()

	ctx, quit := context.WithCancel(ctx)
	defer quit()

	program := tea.NewProgram(tui.New(quit, progR), tea.WithAltScreen())

	tuiDone := make(chan error, 1)
	go func() {
		_, err := program.Run()
		tuiDone <- err
	}()

	iterateOverEvents(journal, func(event *progrock.StatusUpdate) error {
		return progW.WriteStatus(event)
	})
	progW.Close()

	tuiErr := <-tuiDone
	return tuiErr
}

func iterateOverEvents(journal string, cb func(event *progrock.StatusUpdate) error) error {
	f, err := os.Open(journal)
	if err != nil {
		return err
	}

	defer f.Close()

	dec := json.NewDecoder(f)
	for {
		var update progrock.StatusUpdate
		if err := dec.Decode(&update); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		err := cb(&update)
		if err != nil {
			return err
		}
	}

	return nil
}
