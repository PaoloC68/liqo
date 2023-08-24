package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

func main() {
	filepath.WalkDir("./", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		isSymlink := (info.Mode() & fs.ModeSymlink) != 0
		if !isSymlink {
			return nil
		}
		fmt.Println(path)
		fmt.Println(d.Name())
		fmt.Println(d.IsDir())
		fmt.Println(info.Mode())
		fmt.Println()
		return nil
	})
}
