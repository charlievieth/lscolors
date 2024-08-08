package lscolors

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"sort"
	"strings"
)

type ParseError struct {
	Value string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("lscolors: unparsable value for LS_COLORS value: %q", e.Value)
}

var NoColor ColorExtension

type ColorExtension struct {
	Ext string // Extension
	Seq string // Color sequence
}

func (c *ColorExtension) Empty() bool {
	return *c == ColorExtension{}
}

func (c *ColorExtension) MatchExt(name string) bool {
	i := len(name)
	j := len(c.Ext)
	// Fast test for last char to skip strcmp when possible.
	// This yields a ~6x speed up.
	return i > 0 && j > 0 /* BCE */ && j <= i && name[i-1] == c.Ext[j-1] &&
		strings.HasSuffix(name, c.Ext)
}

func (c *ColorExtension) AppendFormat(b []byte, s string) []byte {
	if c.Seq == "" {
		b = slices.Grow(b, len("\x1b[0m")+len(s)+len("\x1b[0m"))
		b = append(b, "\x1b[0m"...)
		b = append(b, s...)
		b = append(b, "\x1b[0m"...)
		return b
	}
	b = slices.Grow(b, len("\x1b[")+len(c.Seq)+len("m")+len(s)+len("\x1b[0m"))
	b = append(b, "\x1b["...)
	b = append(b, c.Seq...)
	b = append(b, 'm')
	b = append(b, s...)
	b = append(b, "\x1b[0m"...)
	return b
}

func (c *ColorExtension) Format(s string) string {
	if c.Seq == "" {
		return "\x1b[0m" + s + "\x1b[0m" // TODO: do we need this?
	}
	return "\x1b[" + c.Seq + "m" + s + "\x1b[0m"
}

// TODO: rename to ColorTerm or something more appropriate
func (e ColorExtension) Raw() string {
	if e.Ext == "" && e.Seq == "" {
		return ""
	}
	return e.Ext + "=*" + e.Seq
}

// func (c *ColorExtension) Sprintf(format string, v ...any) string {
// 	return fmt.Sprintf(c.Format(format), v...)
// }
//
// func (c *ColorExtension) Sprint(v ...any) string {
// 	return c.Format(fmt.Sprint(v...))
// }
//
// func (c *ColorExtension) Sprintln(v ...any) string {
// 	return c.Format(fmt.Sprint(v...)) + "\n" // Add newline after color
// }
//
// func (c *ColorExtension) Fprintf(w io.Writer, format string, v ...any) (int, error) {
// 	return fmt.Fprintf(w, c.Format(format), v...)
// }
//
// func (c *ColorExtension) Fprintln(w io.Writer, v ...any) (int, error) {
// 	return w.Write(c.AppendFormat(nil, c.Sprintln(v...)))
// }
//
// func (c *ColorExtension) Fprint(w io.Writer, v ...any) (int, error) {
// 	return w.Write(c.AppendFormat(nil, c.Sprint(v...)))
// }
//
// func (c *ColorExtension) Printf(format string, v ...any) (int, error) {
// 	return c.Fprintf(os.Stdout, format, v...)
// }
//
// func (c *ColorExtension) Println(v ...any) (int, error) {
// 	return c.Fprintln(os.Stdout, v...)
// }
//
// func (c *ColorExtension) Print(v ...any) (int, error) {
// 	return c.Fprint(os.Stdout, v...)
// }

// From coreutils/ls.c
/*
static struct bin_str color_indicator[] = {
    {LEN_STR_PAIR("\033[")},  // lc: Left of color sequence
    {LEN_STR_PAIR("m")},      // rc: Right of color sequence
    {0, nullptr},             // ec: End color (replaces lc+rs+rc)
    {LEN_STR_PAIR("0")},      // rs: Reset to ordinary colors
    {0, nullptr},             // no: Normal
    {0, nullptr},             // fi: File: default
    {LEN_STR_PAIR("01;34")},  // di: Directory: bright blue
    {LEN_STR_PAIR("01;36")},  // ln: Symlink: bright cyan
    {LEN_STR_PAIR("33")},     // pi: Pipe: yellow/brown
    {LEN_STR_PAIR("01;35")},  // so: Socket: bright magenta
    {LEN_STR_PAIR("01;33")},  // bd: Block device: bright yellow
    {LEN_STR_PAIR("01;33")},  // cd: Char device: bright yellow
    {0, nullptr},             // mi: Missing file: undefined
    {0, nullptr},             // or: Orphaned symlink: undefined
    {LEN_STR_PAIR("01;32")},  // ex: Executable: bright green
    {LEN_STR_PAIR("01;35")},  // do: Door: bright magenta
    {LEN_STR_PAIR("37;41")},  // su: setuid: white on red
    {LEN_STR_PAIR("30;43")},  // sg: setgid: black on yellow
    {LEN_STR_PAIR("37;44")},  // st: sticky: black on blue
    {LEN_STR_PAIR("34;42")},  // ow: other-writable: blue on green
    {LEN_STR_PAIR("30;42")},  // tw: ow w/ sticky: black on green
    {0, nullptr},             // ca: disabled by default
    {0, nullptr},             // mh: disabled by default
    {LEN_STR_PAIR("\033[K")}, // cl: clear to end of line
};
*/

type LSColors struct {
	DI ColorExtension // Directory
	FI ColorExtension // File
	LN ColorExtension // Symbolic Link
	PI ColorExtension // Fifo file
	SO ColorExtension // Socket file
	BD ColorExtension // Block (buffered) special file
	CD ColorExtension // Character (unbuffered) special file
	OR ColorExtension // Symbolic Link pointing to a non-existent file (orphan)
	MI ColorExtension // Non-existent file pointed to by a symbolic link (visible when you type ls -l)
	EX ColorExtension // File which is executable (ie. has 'x' set in permissions).
	TW ColorExtension // ow w/ sticky: black on green

	// NOTE: These are here for correctness but are not currently being used.
	// TODO: Use them.
	NO ColorExtension // Normal
	ST ColorExtension // sticky: black on blue
	OW ColorExtension // other-writable: blue on green

	Exts []ColorExtension
}

func (c LSColors) String() string {
	n := 40 // 40 for all the base colors which need 4 chars each ("di=:")
	for _, e := range []*ColorExtension{
		&c.DI, &c.FI, &c.LN, &c.PI, &c.SO,
		&c.BD, &c.CD, &c.OR, &c.MI, &c.EX,
	} {
		n += len(e.Seq)
	}
	// We strip the '*' from the ext so need to account for that
	n += len(c.Exts) * 3
	for _, e := range c.Exts {
		n += len(e.Ext) + len(e.Seq)
	}
	var w strings.Builder
	w.Grow(n)
	for _, e := range []*ColorExtension{
		&c.DI, &c.FI, &c.LN, &c.PI, &c.SO,
		&c.BD, &c.CD, &c.OR, &c.MI, &c.EX,
	} {
		if len(e.Ext) != 0 && len(e.Seq) != 0 {
			if w.Len() > 0 {
				w.WriteByte(':')
			}
			w.WriteString(e.Ext)
			w.WriteByte('=')
			w.WriteString(e.Seq)
		}
	}
	for _, e := range c.Exts {
		if len(e.Ext) == 0 || len(e.Seq) == 0 {
			continue // this should not happen
		}
		if w.Len() > 0 {
			w.WriteByte(':')
		}
		w.WriteByte('*')
		w.WriteString(e.Ext)
		w.WriteByte('=')
		w.WriteString(e.Seq)
	}
	return w.String()
}

func isBrokenLink(path string, d fs.DirEntry) bool {
	// Check for a fastwalk.DirEntry
	if de, ok := d.(interface{ Stat() (fs.FileInfo, error) }); ok {
		_, err := de.Stat()
		return err != nil
	}
	_, err := os.Stat(path)
	return err != nil
}

func (c *LSColors) MatchEntry(path string, d fs.DirEntry) *ColorExtension {
	var ext *ColorExtension
	typ := d.Type()
	switch {
	case typ.IsDir() && !c.DI.Empty():
		ext = &c.DI
	case typ.IsRegular():
		if typ&0111 != 0 && !c.EX.Empty() {
			ext = &c.EX
		} else if !c.FI.Empty() {
			ext = &c.FI
		}
	case typ&fs.ModeSymlink != 0:
		// TODO: make sure this matches the `ls` broken link logic
		if !c.LN.Empty() {
			ext = &c.LN
		}
		if !c.OR.Empty() && isBrokenLink(path, d) {
			ext = &c.OR
		}
	case typ&fs.ModeNamedPipe != 0 && !c.PI.Empty():
		ext = &c.PI
	case typ&fs.ModeSocket != 0 && !c.PI.Empty():
		ext = &c.PI
	case typ&fs.ModeDevice != 0 && !c.BD.Empty():
		ext = &c.BD
	case typ&fs.ModeCharDevice != 0 && !c.CD.Empty():
		ext = &c.CD
	case typ&0111 != 0 && !c.EX.Empty():
		ext = &c.EX
	default:
		// TODO: GNU ls marks other files as broken links C_ORPHAN
		if !c.OR.Empty() {
			ext = &c.OR
		}
	}
	if typ.IsRegular() && ext != &c.EX {
		if e := c.matchExt(d.Name()); e != nil {
			return e
		}
	}
	if ext == nil {
		return &NoColor
	}
	return ext
}

func (c *LSColors) MatchInfo(path string, d fs.FileInfo) *ColorExtension {
	var ext *ColorExtension
	typ := d.Mode()
	switch {
	case typ.IsDir() && !c.DI.Empty():
		ext = &c.DI
	case typ.IsRegular():
		if typ&0111 != 0 && !c.EX.Empty() {
			ext = &c.EX
		} else if !c.FI.Empty() {
			ext = &c.FI
		}
	case typ&fs.ModeSymlink != 0:
		// TODO: make sure this matches the `ls` broken link logic
		if !c.LN.Empty() {
			ext = &c.LN
		}
		if !c.OR.Empty() && isBrokenLink(path, fs.FileInfoToDirEntry(d)) {
			ext = &c.OR
		}
	case typ&fs.ModeNamedPipe != 0 && !c.PI.Empty():
		ext = &c.PI
	case typ&fs.ModeSocket != 0 && !c.PI.Empty():
		ext = &c.PI
	case typ&fs.ModeDevice != 0 && !c.BD.Empty():
		ext = &c.BD
	case typ&fs.ModeCharDevice != 0 && !c.CD.Empty():
		ext = &c.CD
	default:
		// TODO: GNU ls marks other files as broken links C_ORPHAN
		if !c.OR.Empty() {
			ext = &c.OR
		}
	}
	if typ.IsRegular() && ext != &c.EX {
		if e := c.matchExt(d.Name()); e != nil {
			return e
		}
	}
	if ext == nil {
		return &NoColor
	}
	return ext
}

func (c *LSColors) matchExt(name string) *ColorExtension {
	// TODO: could sort in reverse then use a binary search on length
	// that way the first match is the longest

	// Find longest pattern
	var sfx *ColorExtension
	for i := range c.Exts {
		e := &c.Exts[i]
		if len(e.Ext) > len(name) {
			break
		}
		if e.MatchExt(name) {
			sfx = e
		}
	}
	return sfx
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func validSequence(s string) bool {
	if len(s) == 0 || !isDigit(s[0]) {
		return false
	}
	n := 1
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case isDigit(c):
			n++
			if n > 3 {
				return false
			}
		case c == ';':
			n = 0
		default:
			return false
		}
	}
	return isDigit(s[len(s)-1])
}

func ParseLSColors(clrs string) (*LSColors, error) {
	if clrs == "" {
		return nil, errors.New("ls_colors: empty LS_COLORS argument")
	}
	var invalid []string
	var ls LSColors
	for len(clrs) > 0 {
		var s string
		if i := strings.IndexByte(clrs, ':'); i >= 0 {
			s = clrs[:i]
			clrs = clrs[i+1:]
		} else {
			s = clrs // EOF
			clrs = ""
		}
		k, v, ok := strings.Cut(s, "=")
		if !ok || k == "" || v == "" {
			invalid = append(invalid, s)
			continue
		}
		switch k {
		case "di":
			ls.DI = ColorExtension{Ext: "di", Seq: v}
		case "fi":
			ls.FI = ColorExtension{Ext: "fi", Seq: v}
		case "ln":
			ls.LN = ColorExtension{Ext: "ln", Seq: v}
		case "pi":
			ls.PI = ColorExtension{Ext: "pi", Seq: v}
		case "so":
			ls.SO = ColorExtension{Ext: "so", Seq: v}
		case "bd":
			ls.BD = ColorExtension{Ext: "bd", Seq: v}
		case "cd":
			ls.CD = ColorExtension{Ext: "cd", Seq: v}
		case "or":
			ls.OR = ColorExtension{Ext: "or", Seq: v}
		case "mi":
			ls.MI = ColorExtension{Ext: "mi", Seq: v}
		case "ex":
			ls.EX = ColorExtension{Ext: "ex", Seq: v}
		case "tw":
			ls.TW = ColorExtension{Ext: "tw", Seq: v}
		case "no":
			ls.NO = ColorExtension{Ext: "no", Seq: v}
		case "st":
			ls.ST = ColorExtension{Ext: "st", Seq: v}
		case "ow":
			ls.OW = ColorExtension{Ext: "ow", Seq: v}
		default:
			if ls.Exts == nil {
				// Lazily allocate
				ls.Exts = make([]ColorExtension, 0, strings.Count(clrs, ":")+1)
			}
			if strings.HasPrefix(k, "*") {
				if !validSequence(v) {
					invalid = append(invalid, s)
					continue
				}
				ls.Exts = append(ls.Exts, ColorExtension{
					Ext: k[1:],
					Seq: v,
				})
			} else {
				invalid = append(invalid, s)
			}
		}
	}
	// Sort by length and name to make the order deterministic.
	// Sorting by only length (which is all we really need) is
	// 3x faster but the order is non-deterministic which
	// makes comparing LSColors by the String method impossible.
	sort.Slice(ls.Exts, func(i, j int) bool {
		e1 := ls.Exts[i].Ext
		e2 := ls.Exts[j].Ext
		if len(e1) < len(e2) {
			return true
		}
		if len(e1) > len(e2) {
			return false
		}
		return e1 < e2
	})
	if len(invalid) > 0 {
		return &ls, fmt.Errorf("lscolors: unparsable value for LS_COLORS "+
			"environment variable(s): %q", invalid)
	}
	return &ls, nil
}

// WARN: rename
func NewLSColors() (*LSColors, error) {
	clrs, ok := os.LookupEnv("LS_COLORS")
	if !ok {
		return nil, errors.New("ls_colors: LS_COLORS not set")
	}
	return ParseLSColors(clrs)
}

// func readdir(name string) ([]fs.FileInfo, error) {
// 	f, err := os.Open(name)
// 	if err != nil {
// 		return nil, err
// 	}
// 	fis, err := f.Readdir(-1)
// 	f.Close()
// 	if len(fis) > 0 {
// 		sort.Slice(fis, func(i, j int) bool {
// 			return fis[i].Name() < fis[j].Name()
// 		})
// 	}
// 	return fis, err
// }
