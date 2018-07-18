package main

import (
    "fmt"
    "strings"
    "time"
    "os"
    "os/exec"
    "path/filepath"
    "bufio"
)

type StudyFile struct {
    path string
}

func (studyFile *StudyFile) hasShebang() bool{
    for line := range studyFile.fileLinesGenerator(){
        if line == "*study" {
            return true
        }
    }
    return false
}

func (studyFile *StudyFile) fileLinesGenerator() <- chan string {
	ch := make(chan string)
	go func() {
        file, err := os.Open(studyFile.path)
        if err != nil {
            fmt.Printf("type: %T; value: %q\n", err, err)
        }
        defer file.Close()
        scanner := bufio.NewScanner(file)
        for scanner.Scan() {
            ch <- scanner.Text()
        }
        close(ch)
	}()
	return ch
}

func (manager *StudyFileManager) getFileNames() []string{
    var fileNames []string
    for i := 0; i < len(manager.studyFiles); i++ {
        fileNames = append(fileNames, manager.studyFiles[i].path)
    }
    return fileNames
}

func NewStudyFileManager() *StudyFileManager {
    manager := &StudyFileManager{}
    dir := "/home/leo/org"
    err := filepath.Walk(dir, func(path string, info os.FileInfo, _ error) error {
        if info.IsDir() && info.Name() == "venv" || info.Name() == ".git"  {
            return filepath.SkipDir
        }
        studyFile := StudyFile{path: path}
        if studyFile.hasShebang() {
            manager.studyFiles = append(manager.studyFiles, studyFile)
        }
        return nil
    })
    if err != nil {
        fmt.Printf("error walking the path %q: %v\n", dir, err)
    }
    return manager
}

type StudyFileManager struct {
    studyFiles []StudyFile
}

func (studyFile *StudyFile) getLineNumbersOfSearchTerm(searchTerm string) []int {
    var lineNumbers []int
    lineNumber := 1
    for line := range studyFile.fileLinesGenerator(){
        if strings.Contains(line, searchTerm) {
            lineNumbers = append(lineNumbers, lineNumber)
        }
        lineNumber ++
    }
    return lineNumbers
}

func (manager *StudyFileManager) getSearchMatchesByLine(searchTerm string) map[string][]int {
    if len(manager.studyFiles) > 0 {
        searchMatchesByLine := make(map[string][]int)
        for i := 0; i < len(manager.studyFiles); i++ {
            lineNumbers := manager.studyFiles[i].getLineNumbersOfSearchTerm(searchTerm)
            if len(lineNumbers) > 0 {
                filePath := manager.studyFiles[i].path
                searchMatchesByLine[filePath] = lineNumbers
            }
        }
        return searchMatchesByLine
    }
    return nil
}


func NewStdoutHandler() *StdoutHandler {
    stdoutHandler := &StdoutHandler{}
    stdoutHandler.TERMINAL_SPACE_SEARCH_TERM_LINE = 2
    stdoutHandler.SEARCH_MATCH_TERMINAL_SPACE_START_LINE = 4
    stdoutHandler.SEARCH_MATCH_TERMINAL_SPACE_END_LINE = 34
    return stdoutHandler
}

type StdoutHandler struct {
    TERMINAL_SPACE_SEARCH_TERM_LINE int
    SEARCH_MATCH_TERMINAL_SPACE_START_LINE int
    SEARCH_MATCH_TERMINAL_SPACE_END_LINE int
}


func (handler *StdoutHandler) displaySearchTermInColor(searchTerm string, colorCode string){
    handler.clearTerminalLine(handler.TERMINAL_SPACE_SEARCH_TERM_LINE)
    fmt.Print(colorCode)
    fmt.Println(searchTerm)
    fmt.Print("\u001b[0m")
    handler.placeCursorAtEndOfSearchTerm()
}


func (handler *StdoutHandler) displaySearchTermBeingTyped(searchTerm string){
    BLUE_COLOR_CODE := "\u001b[34m"
    handler.displaySearchTermInColor(searchTerm, BLUE_COLOR_CODE)
}


func (handler *StdoutHandler) displaySearchTermWithMatches(searchTerm string){
    GREEN_COLOR_CODE := "\u001b[32m"
    handler.displaySearchTermInColor(searchTerm, GREEN_COLOR_CODE)
}


func (handler *StdoutHandler) displaySearchTermWithoutMatches(searchTerm string){
    RED_COLOR_CODE := "\u001b[31m"
    handler.displaySearchTermInColor(searchTerm, RED_COLOR_CODE)
}


func (handler *StdoutHandler) clearTerminalLine(numberOfLineToClear int){
    fmt.Printf("\033[%d;1H", numberOfLineToClear)
    fmt.Printf("\033[K")
}

func (handler *StdoutHandler) clearSearchMatchTerminalSpace(){
    for i := handler.SEARCH_MATCH_TERMINAL_SPACE_START_LINE; i <= handler.SEARCH_MATCH_TERMINAL_SPACE_END_LINE; i++ {
        handler.clearTerminalLine(i)
    }
    fmt.Printf("\033[%d;1H", handler.SEARCH_MATCH_TERMINAL_SPACE_START_LINE)
}

func (handler *StdoutHandler) placeCursorAtEndOfSearchTerm(){
    fmt.Printf("\033[%d;100H", handler.TERMINAL_SPACE_SEARCH_TERM_LINE)
}


func (handler *StdoutHandler) displaySearchMatches(linesByFilePath map[string][]int){
    handler.clearSearchMatchTerminalSpace()
    if len(linesByFilePath) > 0 {
        for filePath, _ := range linesByFilePath {
            fmt.Println(filePath)
        }
    }
    handler.placeCursorAtEndOfSearchTerm()
}


func (handler *StdoutHandler) handleSearchMatches(beingTyped string, linesByFilePath map[string][]int) error {
    if len(linesByFilePath) == 0 {
        handler.displaySearchTermWithoutMatches(beingTyped)
    } else {
        handler.displaySearchTermWithMatches(beingTyped)
    }
    handler.displaySearchMatches(linesByFilePath)
    return nil
}

func listenAndSearchForCLIInput() error {

    studyFileManager := *NewStudyFileManager()
    stdoutHandler := *NewStdoutHandler()
    beingTyped := ""
    lastSearched := ""
    DEBOUNCE_TIME_MS := 300

    ch := make(chan []byte)
    go func(ch chan []byte) {
        for {
            exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
            exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
            var b []byte = make([]byte, 1)
            os.Stdin.Read(b)
            ch <- b
        }
        close(ch)
    }(ch)

    stdinLoop:
    for {
        select {
            //you have stdin coming in
            case stdin, ok := <-ch:
                if !ok {
                    break stdinLoop
                } else {
                    //editing beingTyped
                    if 32 <= stdin[0] && stdin[0] <= 126 {
                        beingTyped = beingTyped + string(stdin)
                    } else if stdin[0] == 127 {
                        if len(beingTyped) > 0 {
                            beingTyped = beingTyped[:len(beingTyped)-1]
                        }
                    }
                    stdoutHandler.displaySearchTermBeingTyped(beingTyped)
                }
            //DEBOUNCE_TIME_MS has passed w/o any stdin
            case <-time.After(time.Duration((1000000 * DEBOUNCE_TIME_MS)) * time.Nanosecond):
                //searching beingTyped
                if beingTyped != lastSearched && len(beingTyped) > 0 {
                    linesByFilePath := studyFileManager.getSearchMatchesByLine(beingTyped)
                    stdoutHandler.handleSearchMatches(beingTyped, linesByFilePath)
                    lastSearched = beingTyped
                }
        }
    }
    return nil
}

func main() {
    listenAndSearchForCLIInput()
}

//https://stackoverflow.com/questions/11336048/how-am-i-meant-to-use-filepath-walk-in-go
//http://spf13.com/post/is-go-object-oriented/
//http://ascii-table.com/ansi-escape-sequences.php
//http://www.lihaoyi.com/post/BuildyourownCommandLinewithANSIescapecodes.html
//https://github.com/mgutz/ansi
