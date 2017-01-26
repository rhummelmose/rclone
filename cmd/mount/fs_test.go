// +build linux darwin freebsd

// Test suite for rclonefs

package mount

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"fs"
	_ "fs/all"
	"fstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Globals
var (
	RemoteName      = flag.String("remote", "", "Remote to test with, defaults to local filesystem")
	SubDir          = flag.Bool("subdir", false, "Set to test with a sub directory")
	Verbose         = flag.Bool("verbose", false, "Set to enable logging")
	DumpHeaders     = flag.Bool("dump-headers", false, "Set to dump headers (needs -verbose)")
	DumpBodies      = flag.Bool("dump-bodies", false, "Set to dump bodies (needs -verbose)")
	Individual      = flag.Bool("individual", false, "Make individual bucket/container/directory for each test - much slower")
	LowLevelRetries = flag.Int("low-level-retries", 10, "Number of low level retries")
)

// TestMain drives the tests
func TestMain(m *testing.M) {
	flag.Parse()
	run = newRun()
	rc := m.Run()
	run.Finalise()
	os.Exit(rc)
}

// Run holds the remotes for a test run
type Run struct {
	mountPath    string
	fremote      fs.Fs
	fremoteName  string
	cleanRemote  func()
	umountResult <-chan error
	skip         bool
}

// run holds the master Run data
var run *Run

// newRun initialise the remote mount for testing and returns a run
// object.
//
// r.fremote is an empty remote Fs
//
// Finalise() will tidy them away when done.
func newRun() *Run {
	r := &Run{
		umountResult: make(chan error, 1),
	}

	// Never ask for passwords, fail instead.
	// If your local config is encrypted set environment variable
	// "RCLONE_CONFIG_PASS=hunter2" (or your password)
	*fs.AskPassword = false
	fs.LoadConfig()
	fs.Config.Verbose = *Verbose
	fs.Config.Quiet = !*Verbose
	fs.Config.DumpHeaders = *DumpHeaders
	fs.Config.DumpBodies = *DumpBodies
	fs.Config.LowLevelRetries = *LowLevelRetries
	var err error
	r.fremote, r.fremoteName, r.cleanRemote, err = fstest.RandomRemote(*RemoteName, *SubDir)
	if err != nil {
		log.Fatalf("Failed to open remote %q: %v", *RemoteName, err)
	}

	err = r.fremote.Mkdir("")
	if err != nil {
		log.Fatalf("Failed to open mkdir %q: %v", *RemoteName, err)
	}

	r.mountPath, err = ioutil.TempDir("", "rclonefs-mount")
	if err != nil {
		log.Fatalf("Failed to create mount dir: %v", err)
	}

	// Mount it up
	r.mount()

	return r
}

func (r *Run) mount() {
	log.Printf("mount %q %q", r.fremote, r.mountPath)
	var err error
	r.umountResult, err = mount(r.fremote, r.mountPath)
	if err != nil {
		log.Printf("mount failed: %v", err)
		r.skip = true
	}
	log.Printf("mount OK")
}

func (r *Run) umount() {
	if r.skip {
		log.Printf("FUSE not found so skipping umount")
		return
	}
	log.Printf("Calling fusermount -u %q", r.mountPath)
	err := exec.Command("fusermount", "-u", r.mountPath).Run()
	if err != nil {
		log.Printf("fusermount failed: %v", err)
	}
	log.Printf("Waiting for umount")
	err = <-r.umountResult
	if err != nil {
		log.Fatalf("umount failed: %v", err)
	}
}

func (r *Run) skipIfNoFUSE(t *testing.T) {
	if r.skip {
		t.Skip("FUSE not found so skipping test")
	}
}

// Finalise cleans the remote and unmounts
func (r *Run) Finalise() {
	r.umount()
	r.cleanRemote()
	err := os.RemoveAll(r.mountPath)
	if err != nil {
		log.Printf("Failed to clean mountPath %q: %v", r.mountPath, err)
	}
}

func (r *Run) path(filepath string) string {
	return path.Join(run.mountPath, filepath)
}

type dirMap map[string]struct{}

// Create a dirMap from a string
func newDirMap(dirString string) (dm dirMap) {
	dm = make(dirMap)
	for _, entry := range strings.Split(dirString, "|") {
		if entry != "" {
			dm[entry] = struct{}{}
		}
	}
	return dm
}

// Returns a dirmap with only the files in
func (dm dirMap) filesOnly() dirMap {
	newDm := make(dirMap)
	for name := range dm {
		if !strings.HasSuffix(name, "/") {
			newDm[name] = struct{}{}
		}
	}
	return newDm
}

// reads the local tree into dir
func (r *Run) readLocal(t *testing.T, dir dirMap, filepath string) {
	realPath := r.path(filepath)
	files, err := ioutil.ReadDir(realPath)
	require.NoError(t, err)
	for _, fi := range files {
		name := path.Join(filepath, fi.Name())
		if fi.IsDir() {
			dir[name+"/"] = struct{}{}
			r.readLocal(t, dir, name)
			assert.Equal(t, fi.Mode().Perm(), os.FileMode(dirPerms))
		} else {
			dir[fmt.Sprintf("%s %d", name, fi.Size())] = struct{}{}
			assert.Equal(t, fi.Mode().Perm(), os.FileMode(filePerms))
		}
	}
}

// reads the remote tree into dir
func (r *Run) readRemote(t *testing.T, dir dirMap, filepath string) {
	objs, dirs, err := fs.NewLister().SetLevel(1).Start(r.fremote, filepath).GetAll()
	if err == fs.ErrorDirNotFound {
		return
	}
	require.NoError(t, err)
	for _, obj := range objs {
		dir[fmt.Sprintf("%s %d", obj.Remote(), obj.Size())] = struct{}{}
	}
	for _, d := range dirs {
		name := d.Remote()
		dir[name+"/"] = struct{}{}
		r.readRemote(t, dir, name)
	}
}

// checkDir checks the local and remote against the string passed in
func (r *Run) checkDir(t *testing.T, dirString string) {
	dm := newDirMap(dirString)
	localDm := make(dirMap)
	r.readLocal(t, localDm, "")
	remoteDm := make(dirMap)
	r.readRemote(t, remoteDm, "")
	// Ignore directories for remote compare
	assert.Equal(t, dm.filesOnly(), remoteDm.filesOnly(), "expected vs remote")
	assert.Equal(t, dm, localDm, "expected vs fuse mount")
}

func (r *Run) createFile(t *testing.T, filepath string, contents string) {
	filepath = r.path(filepath)
	err := ioutil.WriteFile(filepath, []byte(contents), 0600)
	require.NoError(t, err)
}

func (r *Run) readFile(t *testing.T, filepath string) string {
	filepath = r.path(filepath)
	result, err := ioutil.ReadFile(filepath)
	require.NoError(t, err)
	return string(result)
}

func (r *Run) mkdir(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := os.Mkdir(filepath, 0700)
	require.NoError(t, err)
}

func (r *Run) rm(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := os.Remove(filepath)
	require.NoError(t, err)
}

func (r *Run) rmdir(t *testing.T, filepath string) {
	filepath = r.path(filepath)
	err := os.Remove(filepath)
	require.NoError(t, err)
}

// Check that the Fs is mounted by seeing if the mountpoint is
// in the mount output
func TestMount(t *testing.T) {
	run.skipIfNoFUSE(t)

	out, err := exec.Command("mount").Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), run.mountPath)
}

// Check root directory is present and correct
func TestRoot(t *testing.T) {
	run.skipIfNoFUSE(t)

	fi, err := os.Lstat(run.mountPath)
	require.NoError(t, err)
	assert.True(t, fi.IsDir())
	assert.Equal(t, fi.Mode().Perm(), os.FileMode(dirPerms))
}
