package reconciler

import (
	"fmt"
	"github.com/mumoshu/crossover/pkg/log"
	"io/ioutil"
	"os"
	"path/filepath"
)

type writer struct {
	xdsDir string

	log.Logger
}

func newWriter(dir string, logger log.Logger) *writer {
	if dir == "" {
		dir = "/srv/runtime"
	}
	return &writer{
		xdsDir: dir,
		Logger: logger,
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
	rf.Infof("Processing %s", id)
	if len(route.Data) == 0 {
		rf.Errorf("Nothing to write! Configmap %s has no data", route.ObjectMeta.Name)
		return nil
	}
	for fn, content := range route.Data {
		newFile := filepath.Join(newDir, fn)
		currentFile := filepath.Join(currentDir, fn)
		rf.Infof("Writing file %s", newFile)
		if err := ioutil.WriteFile(newFile, []byte(content), 0666); err != nil {
			return err
		}
		rf.Infof("Moving file to %s", currentFile)
		if err := os.Rename(newFile, currentFile); err != nil {
			return fmt.Errorf("failed renaming %s to %s: %v", newFile, currentFile, err)
		}
	}

	return nil
}
