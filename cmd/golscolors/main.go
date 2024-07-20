package main

import (
	"bufio"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/charlievieth/fastwalk"
	"github.com/charlievieth/lscolors"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {

	var _ = fastwalk.Config{}
	var _ = lscolors.ColorExtension{}
	root := os.Args[1]
	conf := fastwalk.DefaultConfig.Copy()
	conf.Sort = fastwalk.SortFilesFirst
	conf.Follow = true
	ls, err := lscolors.NewLSColors()
	if err != nil {
		log.Fatal(err)
	}
	var mu sync.Mutex
	bw := bufio.NewWriterSize(os.Stdout, 32*1024)
	err = fastwalk.Walk(conf, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// if strings.HasPrefix(d.Name(), ".") {
		// 	return nil
		// }
		if d.IsDir() && d.Name() == ".git" {
			return fastwalk.SkipDir
		}
		dir, base := filepath.Split(path)
		c := ls.MatchEntry(path, d)
		mu.Lock()
		bw.WriteString(ls.DI.Format(dir))
		bw.WriteString(c.Format(base))
		err = bw.WriteByte('\n')
		mu.Unlock()
		return err
	})
	if err != nil {
		log.Panic(err)
	}
	if err := bw.Flush(); err != nil {
		log.Panic(err)
	}
}
