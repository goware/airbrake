// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Some code from the runtime/debug package of the Go standard library.
// Some code from sentry go-raven

package airbrake

import (
	"bytes"
	"go/build"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Stacktrace struct {
	Frames []*StacktraceFrame `json:"frames"`
}

func (s *Stacktrace) Culprit() string {
	culprit := ""
	for i := 0; i < len(s.Frames); i++ {
		// Default to 1st frame with non-empty package name and function set
		if culprit == "" {
			if s.Frames[i].Package != "" && s.Frames[i].Function != "" {
				culprit = s.Frames[i].Package + "." + s.Frames[i].Function
			}
		}

		// Frames in app paths (if you provided any) take precedence
		if s.Frames[i].InApp == true {
			if s.Frames[i].Package != "" && s.Frames[i].Function != "" {
				culprit = s.Frames[i].Package + "." + s.Frames[i].Function
				break
			}
		}
	}
	return culprit
}

type StacktraceFrame struct {
	// At least one required
	Filename string `json:"file"`
	Function string `json:"function"`
	Line     int    `json:"line"`
	Package  string `json:"package"`

	// Optional
	ContextLine string
	PreContext  []string
	PostContext []string
	InApp       bool
}

// Intialize and populate a new stacktrace, skipping skip frames.
//
// context is the number of surrounding lines that should be included for context.
// Setting context to 3 would try to get seven lines. Setting context to -1 returns
// one line with no surrounding context, and 0 returns no context.
//
// appPackagePrefixes is a list of prefixes used to check whether a package should
// be considered "in app".
func NewStacktrace(skip int, context int, appPackagePrefixes []string) *Stacktrace {
	var frames []*StacktraceFrame
	for i := 1 + skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		frame := NewStacktraceFrame(pc, file, line, context, appPackagePrefixes)
		if frame != nil {
			frames = append(frames, frame)
		}
	}
	return &Stacktrace{frames}
}

// Build a single frame using data returned from runtime.Caller.
//
// context is the number of surrounding lines that should be included for context.
// Setting context to 3 would try to get seven lines. Setting context to -1 returns
// one line with no surrounding context, and 0 returns no context.
//
// appPackagePrefixes is a list of prefixes used to check whether a package should
// be considered "in app".
func NewStacktraceFrame(pc uintptr, file string, line, context int, appPackagePrefixes []string) *StacktraceFrame {
	frame := &StacktraceFrame{Filename: trimPath(file), Line: line, InApp: false}
	frame.Package, frame.Function = functionName(pc)

	// `runtime.goexit` is effectively a placeholder that comes from
	// runtime/asm_amd64.s and is meaningless.
	if frame.Package == "runtime" && frame.Function == "goexit" {
		return nil
	}

	if frame.Package == "runtime" && frame.Function == "gopanic" {
		return nil
	}

	if frame.Package == "main" {
		frame.InApp = true
	} else {
		for _, prefix := range appPackagePrefixes {
			if strings.HasPrefix(frame.Package, prefix) && !strings.Contains(frame.Package, "vendor") && !strings.Contains(frame.Package, "third_party") {
				frame.InApp = true
			}
		}
	}

	if context > 0 {
		contextLines, lineIdx := fileContext(file, line, context)
		if len(contextLines) > 0 {
			for i, line := range contextLines {
				switch {
				case i < lineIdx:
					frame.PreContext = append(frame.PreContext, string(line))
				case i == lineIdx:
					frame.ContextLine = string(line)
				default:
					frame.PostContext = append(frame.PostContext, string(line))
				}
			}
		}
	} else if context == -1 {
		contextLine, _ := fileContext(file, line, 0)
		if len(contextLine) > 0 {
			frame.ContextLine = string(contextLine[0])
		}
	}
	return frame
}

// Retrieve the name of the package and function containing the PC.
func functionName(pc uintptr) (pack string, name string) {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return
	}
	name = fn.Name()
	// We get this:
	//	runtime/debug.*T·ptrmethod
	// and want this:
	//  pack = runtime/debug
	//	name = *T.ptrmethod
	if idx := strings.LastIndex(name, "."); idx != -1 {
		pack = name[:idx]
		name = name[idx+1:]
	}
	name = strings.Replace(name, "·", ".", -1)
	return
}

var fileCacheLock sync.Mutex
var fileCache = make(map[string][][]byte)

func fileContext(filename string, line, context int) ([][]byte, int) {
	if context < 0 {
		return nil, 0
	}

	fileCacheLock.Lock()
	defer fileCacheLock.Unlock()
	lines, ok := fileCache[filename]
	if !ok {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, 0
		}
		lines = bytes.Split(data, []byte{'\n'})
		fileCache[filename] = lines
	}
	line-- // stack trace lines are 1-indexed
	start := line - context
	var idx int
	if start < 0 {
		start = 0
		idx = line
	} else {
		idx = context
	}
	end := line + context + 1
	if line >= len(lines) {
		return nil, 0
	}
	if end > len(lines) {
		end = len(lines)
	}
	return lines[start:end], idx
}

var sourcePaths []string

// Try to trim the GOROOT or GOPATH prefix off of a filename
func trimPath(filename string) string {
	for _, prefix := range sourcePaths {
		if trimmed := strings.TrimPrefix(filename, prefix); len(trimmed) < len(filename) {
			return trimmed
		}
	}
	return filename
}

func init() {
	// Collect all source directories, and make sure they
	// end in a trailing "separator"
	for _, prefix := range build.Default.SrcDirs() {
		if prefix[len(prefix)-1] != filepath.Separator {
			prefix += string(filepath.Separator)
		}
		sourcePaths = append(sourcePaths, prefix)
	}
}
