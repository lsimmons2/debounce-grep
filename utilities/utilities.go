package utilities

import (
    "math"
    "fmt"
    "github.com/maxmclau/gput"
    "log"
    "os"
    "path/filepath"
    "io/ioutil"
)

const (
    //environmental variables for config options
    DEBOUNCE_GREP_LOG_FILE_PATH = "DEBOUNCE_GREP_LOG_FILE_PATH"
)

func SetUpLogging() {
    logFilePath := os.Getenv(DEBOUNCE_GREP_LOG_FILE_PATH)
    if len(logFilePath) == 0 {
        log.SetFlags(0)
        log.SetOutput(ioutil.Discard)
        return
    }
    f, err := os.OpenFile(logFilePath, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("error opening file: %v", err)
    }
    //defer f.Close()
    log.SetOutput(f)
}

//this can be done with math.Round in go 1.10
//this function rounds away from 0 if float ends in .5
func Round(x float64) int {
    t := math.Trunc(x)
    if math.Abs(x-t) >= 0.5 {
        return int(t + math.Copysign(1, x))
    }
    return int(t)
}

func PrintNewLine() {
    fmt.Println("")
}

func GetTtyDimensions() (int, int) {
    lines := gput.Lines()
    cols := gput.Cols()
    log.Printf("Detected tty dimensions: %v x %v.", lines, cols)
    return lines, cols
}

func GetDirsToSearch() []string {
    //looks first at cli args passed (not as flags starting with - or --),
    //if no dirs are passed as cli args this way, then returns just the cwd
    cwd := GetCurrentWorkingDir()
    if len(os.Args[1:]) > 0 {
        var dirs []string
        var dir string
        var err error
        for _, arg := range os.Args[1:]{
            dir,err = filepath.Abs(arg)
            if err != nil {
                panic(fmt.Sprintf("Could not resolve directory %s passed as CLI arg", dir))
            }
            dirs = append(dirs,dir)
        }
        return dirs
    }
    return []string{cwd}
}

func GetCurrentWorkingDir() string{
    currentWorkingDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
    return currentWorkingDir
}

func init(){
    SetUpLogging()
}
