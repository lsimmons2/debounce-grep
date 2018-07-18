package main

import (
    "fmt"
    "strings"
    "time"
    "os"
    "os/exec"
    "path/filepath"
    "bufio"
    "log"
    "strconv"
)

type FileToSearch struct {
    path string
}

func (FileToSearch *FileToSearch) hasShebang() bool{
    for line := range FileToSearch.fileLinesGenerator(){
        if line == "*study" {
            return true
        }
    }
    return false
}

func (FileToSearch *FileToSearch) fileLinesGenerator() <- chan string {
	ch := make(chan string)
	go func() {
        file, err := os.Open(FileToSearch.path)
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

func (FileToSearch *FileToSearch) getLineNumbersOfSearchTerm(searchTerm string) []int {
    var lineNumbers []int
    lineNumber := 1
    for line := range FileToSearch.fileLinesGenerator(){
        if strings.Contains(line, searchTerm) {
            lineNumbers = append(lineNumbers, lineNumber)
        }
        lineNumber ++
    }
    return lineNumbers
}



type FileSearcher struct {
    studyFiles []FileToSearch
}

func NewFileSearcher() *FileSearcher {
    fileSearcher := &FileSearcher{}
    dir := "/home/leo/org"
    err := filepath.Walk(dir, func(path string, info os.FileInfo, _ error) error {
        if info.IsDir() && info.Name() == "venv" || info.Name() == ".git"  {
            return filepath.SkipDir
        }
        fileToSearch := FileToSearch{path: path}
        if fileToSearch.hasShebang() {
            fileSearcher.studyFiles = append(fileSearcher.studyFiles, fileToSearch)
        }
        return nil
    })
    if err != nil {
        fmt.Printf("error walking the path %q: %v\n", dir, err)
    }
    return fileSearcher
}

func (fileSearcher *FileSearcher) getFileNames() []string{
    var fileNames []string
    for i := 0; i < len(fileSearcher.studyFiles); i++ {
        fileNames = append(fileNames, fileSearcher.studyFiles[i].path)
    }
    return fileNames
}

func (fileSearcher *FileSearcher) getSearchMatchesByLine(searchTerm string) map[string][]int {
    if len(fileSearcher.studyFiles) > 0 {
        searchMatchesByLine := make(map[string][]int)
        for i := 0; i < len(fileSearcher.studyFiles); i++ {
            lineNumbers := fileSearcher.studyFiles[i].getLineNumbersOfSearchTerm(searchTerm)
            if len(lineNumbers) > 0 {
                filePath := fileSearcher.studyFiles[i].path
                searchMatchesByLine[filePath] = lineNumbers
            }
        }
        return searchMatchesByLine
    }
    return nil
}


type SearchManager struct {
    fileSearcher FileSearcher
    DEBOUNCE_TIME_MS int
    TERMINAL_SPACE_SEARCH_TERM_LINE int
    SEARCH_MATCH_TERMINAL_SPACE_START_LINE int
    SEARCH_MATCH_TERMINAL_SPACE_END_LINE int
    cursorIndex int
    searchTerm string
}

func NewSearchManager() *SearchManager {
    searchManager := &SearchManager{}
    searchManager.DEBOUNCE_TIME_MS = 300
    searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE = 2
    searchManager.SEARCH_MATCH_TERMINAL_SPACE_START_LINE = 4
    searchManager.SEARCH_MATCH_TERMINAL_SPACE_END_LINE = 34
    searchManager.cursorIndex = 0
    searchManager.searchTerm = ""
    return searchManager
}

func (searchManager *SearchManager) listenToStdinAndSearchFiles() error {

    //beingTyped := ""
    //lastSearched := ""

    stdinChannel := make(chan []byte)
    go func(stdinChannel chan []byte) {
        for {
            exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
            exec.Command("stty", "-F", "/dev/tty", "-echo").Run()
            var b []byte = make([]byte, 1)
            os.Stdin.Read(b)
            stdinChannel <- b
        }
        close(stdinChannel)
    }(stdinChannel)

    stdinLoop:
    for {
        select {
            //stdin coming in
            case stdin, ok := <-stdinChannel:
                if !ok {
                    break stdinLoop
                } else {
                    searchManager.editSearchTermWithStdin(stdin)
                }
            //DEBOUNCE_TIME_MS has passed w/o any stdin
            case <-time.After(time.Duration((1000000 * searchManager.DEBOUNCE_TIME_MS)) * time.Nanosecond):
                //searching beingTyped
                //if beingTyped != lastSearched && len(beingTyped) > 0 {
                    //linesByFilePath := searchManager.fileSearcher.getSearchMatchesByLine(beingTyped)
                    //searchManager.handleSearchMatches(beingTyped, linesByFilePath)
                    //lastSearched = beingTyped
                //}
        }
    }
    return nil
}

func (searchManager *SearchManager) positionCursorAtIndex(){
    fmt.Printf("\033[%d;%dH", searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE, searchManager.cursorIndex+1)
}

func (searchManager *SearchManager) displaySearchTermInColor(colorCode string){
    searchManager.clearTerminalLine(searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE)
    fmt.Print(colorCode)
    fmt.Print(searchManager.searchTerm)
    fmt.Print("\u001b[0m")
    searchManager.positionCursorAtIndex()
}

func (searchManager *SearchManager) renderSearchTermBeingTyped(){
    BLUE_COLOR_CODE := "\u001b[34m"
    searchManager.displaySearchTermInColor(BLUE_COLOR_CODE)
}

func (searchManager *SearchManager) displayPositiveSearchTerm(){
    GREEN_COLOR_CODE := "\u001b[32m"
    searchManager.displaySearchTermInColor(GREEN_COLOR_CODE)
}

func (searchManager *SearchManager) displayNegativeSearchTerm(){
    RED_COLOR_CODE := "\u001b[31m"
    searchManager.displaySearchTermInColor(RED_COLOR_CODE)
}

func (searchManager *SearchManager) clearTerminalLine(numberOfLineToClear int){
    fmt.Printf("\033[%d;1H", numberOfLineToClear)
    fmt.Printf("\033[K")
}

func (searchManager *SearchManager) clearSearchMatchTerminalSpace(){
    for i := searchManager.SEARCH_MATCH_TERMINAL_SPACE_START_LINE; i <= searchManager.SEARCH_MATCH_TERMINAL_SPACE_END_LINE; i++ {
        searchManager.clearTerminalLine(i)
    }
    fmt.Printf("\033[%d;1H", searchManager.SEARCH_MATCH_TERMINAL_SPACE_START_LINE)
}

func (searchManager *SearchManager) displaySearchMatches(linesByFilePath map[string][]int){
    searchManager.clearSearchMatchTerminalSpace()
    if len(linesByFilePath) > 0 {
        for filePath, _ := range linesByFilePath {
            fmt.Println(filePath)
        }
    }
}

func (searchManager *SearchManager) handleSearchMatches(searchTerm string, linesByFilePath map[string][]int) error {
    if len(linesByFilePath) == 0 {
        searchManager.displayNegativeSearchTerm()
    } else {
        searchManager.displayPositiveSearchTerm()
    }
    return nil
}

func (searchManager *SearchManager) incrementCursorIndex() {
    searchManager.cursorIndex += 1
}

func (searchManager *SearchManager) decrementCursorIndex() {
    searchManager.cursorIndex -= 1
}

func (searchManager *SearchManager) deleteCharBackwards() {
    searchManager.searchTerm = searchManager.searchTerm[0:searchManager.cursorIndex-1] + searchManager.searchTerm[searchManager.cursorIndex:]
}

func (searchManager *SearchManager) deleteCharForwards() {
    searchManager.searchTerm = searchManager.searchTerm[0:searchManager.cursorIndex] + searchManager.searchTerm[searchManager.cursorIndex+1:]
}

func (searchManager *SearchManager) addCharToSearchTerm(char string) {
    if searchManager.cursorIndex == 0 {
        searchManager.searchTerm = char + searchManager.searchTerm
        logToFileCursor(searchManager.cursorIndex)
    } else if searchManager.cursorIndex == 1 {
        searchManager.searchTerm = searchManager.searchTerm + char
    } else {
        logToFileCursor(searchManager.cursorIndex)
        searchManager.searchTerm = searchManager.searchTerm[:searchManager.cursorIndex] + char + searchManager.searchTerm[searchManager.cursorIndex:]
    }
    searchManager.incrementCursorIndex()
}

func (searchManager *SearchManager) editSearchTermWithStdin(stdin []byte) {
    if 32 <= stdin[0] && stdin[0] <= 126 { // char is alphanumeric or punctuation
        searchManager.addCharToSearchTerm(string(stdin))
        searchManager.renderSearchTermBeingTyped()

    } else if stdin[0] == 6 { // C-f
        searchManager.incrementCursorIndex()
        searchManager.renderSearchTermBeingTyped()

    } else if stdin[0] == 2 { // C-b
        searchManager.decrementCursorIndex()
        searchManager.renderSearchTermBeingTyped()

    } else if stdin[0] == 4 { // delete forewards one char
        searchManager.deleteCharForwards()
        searchManager.renderSearchTermBeingTyped()

    } else if stdin[0] == 127 { // delete backwards one char
        searchManager.deleteCharBackwards()
        searchManager.decrementCursorIndex()
        searchManager.renderSearchTermBeingTyped()
        //if len(searchManager.searchTerm) > 0 {
            ////searchManager.searchTerm = searchManager.searchTerm[:len(searchManager.searchTerm)-1]
            //searchManager.searchTerm = searchManager.searchTerm[0:searchManager.cursorIndex-2] + searchManager.searchTerm[searchManager.cursorIndex:]
            //searchManager.decrementCursorIndex()
        //}
    }
    //return searchTerm
}

func main() {
    searchManager := NewSearchManager()
    searchManager.listenToStdinAndSearchFiles()
}

//https://stackoverflow.com/questions/11336048/how-am-i-meant-to-use-filepath-walk-in-go
//http://spf13.com/post/is-go-object-oriented/
//http://ascii-table.com/ansi-escape-sequences.php
//http://www.lihaoyi.com/post/BuildyourownCommandLinewithANSIescapecodes.html
//https://github.com/mgutz/ansi


func logToFile(message string) {
    file, err := os.OpenFile("/home/leo/go/src/notes_searcher/log.log", os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil {
        log.Fatal("Cannot create file", err)
    }
    defer file.Close()
    fmt.Fprintln(file, message)
}

func logToFileCursor(index int) {
    file, err := os.OpenFile("/home/leo/go/src/notes_searcher/log.log", os.O_APPEND|os.O_WRONLY, 0600)
    if err != nil {
        log.Fatal("Cannot create file", err)
    }
    defer file.Close()
    fmt.Fprintln(file, strconv.Itoa(index))
}
