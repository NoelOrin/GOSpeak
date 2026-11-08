package main

import (
	"context"

	"github.com/egoist/mygo"
)

// OpenDialogOptions / SaveDialogOptions / MessageBoxOptions mirror the dialog
// API in packages/desktop. `filters` arrives as a name → extensions map and is
// converted to mygo.FileFilter below.
type OpenDialogOptions struct {
	Title    string              `json:"title,omitempty"`
	Filters  map[string][]string `json:"filters,omitempty"`
	Multiple bool                `json:"multiple,omitempty"`
}

type SaveDialogOptions struct {
	Title       string              `json:"title,omitempty"`
	DefaultPath string              `json:"defaultPath,omitempty"`
	Filters     map[string][]string `json:"filters,omitempty"`
}

type MessageBoxOptions struct {
	Title   string   `json:"title,omitempty"`
	Message string   `json:"message"`
	Kind    string   `json:"kind,omitempty"`
	Buttons []string `json:"buttons,omitempty"`
}

// toMygoFilters converts the TS filter map { "Audio files": ["wav","mp3"] }
// into mygo's FileFilter list.
func toMygoFilters(fs map[string][]string) []mygo.FileFilter {
	if len(fs) == 0 {
		return nil
	}
	out := make([]mygo.FileFilter, 0, len(fs))
	for name, exts := range fs {
		out = append(out, mygo.FileFilter{Name: name, Extensions: exts})
	}
	return out
}

func messageType(kind string) mygo.MessageType {
	switch kind {
	case "warning":
		return mygo.MessageWarning
	case "error":
		return mygo.MessageError
	case "question":
		return mygo.MessageQuestion
	case "info":
		return mygo.MessageInfo
	default:
		return mygo.MessageNone
	}
}

// Dialog exposes native file/system dialogs (mygo.DialogModule), parented to
// the calling window when available.
type Dialog struct{}

func (Dialog) Open(ctx context.Context, options OpenDialogOptions) ([]string, error) {
	return mygo.DialogModule{}.Open(mygo.OpenDialogOptions{
		Title:    options.Title,
		Filters:  toMygoFilters(options.Filters),
		Multiple: options.Multiple,
		Parent:   mygo.CallerWindow(ctx),
	})
}

func (Dialog) Save(ctx context.Context, options SaveDialogOptions) (string, error) {
	return mygo.DialogModule{}.Save(mygo.SaveDialogOptions{
		Title:       options.Title,
		DefaultPath: options.DefaultPath,
		Filters:     toMygoFilters(options.Filters),
		Parent:      mygo.CallerWindow(ctx),
	})
}

func (Dialog) Message(ctx context.Context, options MessageBoxOptions) (int, error) {
	res, err := mygo.DialogModule{}.Message(mygo.MessageOptions{
		Title:   options.Title,
		Message: options.Message,
		Type:    messageType(options.Kind),
		Buttons: options.Buttons,
		Parent:  mygo.CallerWindow(ctx),
	})
	if err != nil {
		return 0, err
	}
	return res.Button, nil
}
