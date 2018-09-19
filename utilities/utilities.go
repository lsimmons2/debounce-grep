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

func SetUpLogging() int{
    logFilePath := os.Getenv(DEBOUNCE_GREP_LOG_FILE_PATH)
    if len(logFilePath) == 0 {
        log.SetFlags(0)
        log.SetOutput(ioutil.Discard)
        return 1
    }
    f, err := os.OpenFile(logFilePath, os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("error opening file: %v", err)
    }
    //defer f.Close()
    log.SetOutput(f)
    return 0
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

func GetCurrentWorkingDir() string{
    currentWorkingDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
    return currentWorkingDir
}

func ReverseStrings(sliceOfStrings []string) {
    last := len(sliceOfStrings) - 1
    for i := 0; i < len(sliceOfStrings)/2; i++ {
        sliceOfStrings[i], sliceOfStrings[last-i] = sliceOfStrings[last-i], sliceOfStrings[i]
    }
}

func init(){
    SetUpLogging()
}
