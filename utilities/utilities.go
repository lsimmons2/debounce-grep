package utilities

import (
    "math"
    "fmt"
    "github.com/maxmclau/gput"
    "log"
    "os"
    "os/user"
    "strconv"
    "strings"
    "github.com/mattn/go-zglob"
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

func GetDirToSearch() string {
    dirToSearchEnvVariable := os.Getenv("DEBOUNCE_GREP_DIR_TO_SEARCH")
    //check if dir exists
    _, err := os.Stat(dirToSearchEnvVariable)
    if err == nil {
        return dirToSearchEnvVariable
    }
    usr, _ := user.Current()
    return usr.HomeDir
}

func GetDebounceTimeMS() int {
    //default debounce time is 200 ms
    var debounceTimeMs int
    debounceTimeMsEnvVariable := os.Getenv("DEBOUNCE_GREP_DEBOUNCE_TIME_MS")
    if len(debounceTimeMsEnvVariable) == 0 {
        return 200
    }
    debounceTimeMs, err := strconv.Atoi(debounceTimeMsEnvVariable)
    if err != nil {
        fmt.Println("DEBOUNCE_GREP_DEBOUNCE_TIME_MS environmental variable was not able to be converted into type int, defaulting to value 200.")
        return 200
    }
    return debounceTimeMs
}

func getEnvVariableList(envVariableName string) []string {
    envVariable := os.Getenv(envVariableName)
    if len(envVariable) == 0 {
        return nil
    }
    return strings.Split(envVariable, ":")
}

func GetFileShebangs() []string {
    return getEnvVariableList("DEBOUNCE_GREP_FILE_SHEBANG")
}

func GetFullPathsToIgnore() []string {
    dirsToSearch := GetDirsToSearch()
    toIgnorePatterns := getEnvVariableList("DEBOUNCE_GREP_FILES_DIRS_TO_IGNORE")
    var toIgnorePaths []string
    for _, dirToSearch := range dirsToSearch {
        for _, toIgnorePattern := range toIgnorePatterns {
            toIgnoreMatches, _ := zglob.Glob(dirToSearch + "/" + toIgnorePattern)
            toIgnorePaths = append(toIgnorePaths, toIgnoreMatches...)
        }
    }
    return toIgnorePaths
}

func GetDirsToSearch() []string {
    var dirsToSearchFromEnv []string
    dirsToSearchFromEnv = getEnvVariableList("DEBOUNCE_GREP_FILES_DIRS_TO_SEARCH")
    if len(dirsToSearchFromEnv) == 0 {
        cwd, _ := filepath.Abs(filepath.Dir(os.Args[0]))
        dirsToSearchFromEnv = append(dirsToSearchFromEnv, cwd)
    }
    return dirsToSearchFromEnv
}

func init(){
    SetUpLogging()
}
