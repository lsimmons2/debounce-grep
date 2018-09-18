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

const (
    //environmental variables for config options
    DEBOUNCE_GREP_DEBOUNCE_TIME_MS = "DEBOUNCE_GREP_DEBOUNCE_TIME_MS"
    DEBOUNCE_GREP_DIRS_TO_SEARCH = "DEBOUNCE_GREP_DIRS_TO_SEARCH"
    DEBOUNCE_GREP_FILE_SHEBANGS = "DEBOUNCE_GREP_FILE_SHEBANGS"
    DEBOUNCE_GREP_LOG_FILE_PATH = "DEBOUNCE_GREP_LOG_FILE_PATH"
    DEBOUNCE_GREP_MAX_LINES_PER_FILE = "DEBOUNCE_GREP_MAX_LINES_PER_FILE"
    DEBOUNCE_GREP_PATTERNS_TO_IGNORE = "DEBOUNCE_GREP_PATTERNS_TO_IGNORE"
    DEBOUNCE_GREP_SHOULD_TRUNCATE_MATCHED_LINES = "DEBOUNCE_GREP_SHOULD_TRUNCATE_MATCHED_LINES"
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
    defaultMaxLines := 5
    return getIntEnvVariable(DEBOUNCE_GREP_MAX_LINES_PER_FILE, defaultMaxLines)
}

func GetDebounceTimeMS() int {
    defaultDebounceTimeMs := 200
    return getIntEnvVariable(DEBOUNCE_GREP_DEBOUNCE_TIME_MS, defaultDebounceTimeMs)
}

func getEnvVariableList(envVariableName string, defaultValues []string) []string {
    envValue := os.Getenv(envVariableName)
    if len (envValue) == 0 {
        log.Printf("Returning default value %v for %v config option.", defaultValues, envVariableName)
        return defaultValues
    }
    envVariableList := strings.Split(envValue, ":")
    log.Printf("Returning environmental variable value %v for %v config option.", envVariableList, envVariableName)
    return envVariableList
}

func GetFileShebangs() []string {
    var nilFileShebangs []string
    return getEnvVariableList(DEBOUNCE_GREP_FILE_SHEBANGS, nilFileShebangs)
}

func GetDirsToSearch() []string {
    currentWorkingDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
    defaultDirsToSearch := []string{currentWorkingDir}
    return getEnvVariableList(DEBOUNCE_GREP_DIRS_TO_SEARCH, defaultDirsToSearch)
}

func GetPatternsToIgnore() []string {
    defaultPatternsToIgnore := []string{".git", "venv", "node_modules", "bower_components", "*.png", "*.jpg", "*.jpeg", "*.pyc"}
    return getEnvVariableList(DEBOUNCE_GREP_PATTERNS_TO_IGNORE, defaultPatternsToIgnore)
}

func GetShouldTruncateMatchedLines() bool {
     shouldTruncateMatchedLines := os.Getenv(DEBOUNCE_GREP_SHOULD_TRUNCATE_MATCHED_LINES)
     if strings.ToLower(shouldTruncateMatchedLines) == "false" {
         return false
     }
     return true
}

func init(){
    SetUpLogging()
}
