package reconciler

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

type writer struct {
	xdsDir string
}

func newWriter(dir string) *writer {
	if dir == "" {
		dir = "/srv/runtime"
	}
	return &writer{
		xdsDir: dir,
	}
}

func (rf *writer) write(route ConfigMap) error {
	newDir := filepath.Join(rf.xdsDir, "new")
	currentDir := filepath.Join(rf.xdsDir, "current")

	if err := os.MkdirAll(newDir, 0777); err != nil {
		return fmt.Errorf("creating dir %s: %v", newDir, err)
	}
	if err := os.MkdirAll(currentDir, 0777); err != nil {
		return fmt.Errorf("creating dir %s: %v", currentDir, err)
	}

	id := fmt.Sprintf("%s/%s", route.ObjectMeta.Namespace, route.ObjectMeta.Name)
	log.SetOutput(os.Stdout)
	log.Printf("Processing %s", id)
	log.SetOutput(os.Stderr)
	if len(route.Data) == 0 {
		log.Printf("Nothing to write! Configmap %s has no data", route.ObjectMeta.Name)
		return nil
	}
	for fn, content := range route.Data {
		newFile := filepath.Join(newDir, fn)
		currentFile := filepath.Join(currentDir, fn)
		log.SetOutput(os.Stdout)
		log.Printf("Writing file %s", newFile)
		log.SetOutput(os.Stderr)
		if err := ioutil.WriteFile(newFile, []byte(content), 0666); err != nil {
			return err
		}
		log.SetOutput(os.Stdout)
		log.Printf("Moving file to %s", currentFile)
		log.SetOutput(os.Stderr)
		if err := os.Rename(newFile, currentFile); err != nil {
			return fmt.Errorf("failed renaming %s to %s: %v", newFile, currentFile, err)
		}
	}

	return nil
}
