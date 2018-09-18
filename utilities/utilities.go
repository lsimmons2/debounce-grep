package utilities

import (
    "math"
    "fmt"
    "github.com/maxmclau/gput"
    "log"
    "os"
    "strconv"
    "strings"
    "path/filepath"
    "io/ioutil"
)

func SetUpLogging() int{
    logFilePath := os.Getenv("DEBOUNCE_GREP_LOG_FILE_PATH")
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

func getIntEnvVariable(envVariableName string, defaultValue int) int {
    envVarValueString := os.Getenv(envVariableName)
    envVarValueInt, err := strconv.Atoi(envVarValueString)
    if err != nil {
        log.Printf("%v environmental variable was not able to be converted into type int, defaulting to value %v.\n", envVariableName, defaultValue)
        return defaultValue
    }
    return envVarValueInt
}

func GetMaxLinesToPrintPerFile() int {
    //default lines to show is 5
    return getIntEnvVariable("DEBOUNCE_GREP_MAX_LINES_PER_FILE", 5)
}

func GetDebounceTimeMS() int {
    //default debounce time is 200 ms
    return getIntEnvVariable("DEBOUNCE_GREP_DEBOUNCE_TIME_MS", 200)
}

func getEnvVariableList(envVariableName string, defaultValues []string) []string {
    envVariable := os.Getenv(envVariableName)
    if len(envVariable) == 0 {
        return defaultValues
    }
    //TODO: should split before returning default value
    return strings.Split(envVariable, ":")
}

func GetFileShebangs() []string {
    return getEnvVariableList("DEBOUNCE_GREP_FILE_SHEBANGS", []string{""})
}

func GetDirsToSearch() []string {
    currentWorkingDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
    defaultDirsToSearch := []string{currentWorkingDir}
    return getEnvVariableList("DEBOUNCE_GREP_FILES_DIRS_TO_SEARCH", defaultDirsToSearch)
}

func GetToIgnorePatterns() []string {
    defaultDirsToIgnore := []string{".git", "venv", "node_modules", "bower_components", "*.png", "*.jpg", "*.jpeg", "*.pyc"}
    return getEnvVariableList("DEBOUNCE_GREP_FILES_DIRS_TO_IGNORE", defaultDirsToIgnore)
}

func GetShouldTruncateMatchedLines() bool {
     shouldTruncateMatchedLines := os.Getenv("DEBOUNCE_GREP_TRUNCATE_MATCHED_LINES")
     if strings.ToLower(shouldTruncateMatchedLines) == "false" {
         return false
     }
     return true
}

func init(){
    SetUpLogging()
}
