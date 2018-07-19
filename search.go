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
    filesToSearch []FileToSearch
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
            fileSearcher.filesToSearch = append(fileSearcher.filesToSearch, fileToSearch)
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
    for i := 0; i < len(fileSearcher.filesToSearch); i++ {
        fileNames = append(fileNames, fileSearcher.filesToSearch[i].path)
    }
    return fileNames
}

func (fileSearcher *FileSearcher) getSearchMatchesByLine(searchTerm string) map[string][]int {
    if len(fileSearcher.filesToSearch) > 0  && len(searchTerm) > 0 {
        searchMatchesByLine := make(map[string][]int)
        for i := 0; i < len(fileSearcher.filesToSearch); i++ {
            lineNumbers := fileSearcher.filesToSearch[i].getLineNumbersOfSearchTerm(searchTerm)
            if len(lineNumbers) > 0 {
                filePath := fileSearcher.filesToSearch[i].path
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
    searchState string
}

func NewSearchManager() *SearchManager {
    searchManager := &SearchManager{}
    searchManager.DEBOUNCE_TIME_MS = 300
    searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE = 2
    searchManager.SEARCH_MATCH_TERMINAL_SPACE_START_LINE = 4
    searchManager.SEARCH_MATCH_TERMINAL_SPACE_END_LINE = 34
    searchManager.cursorIndex = 0
    searchManager.searchTerm = ""
    searchManager.searchState = "TYPING"
    searchManager.fileSearcher = *NewFileSearcher()
    return searchManager
}

func (searchManager *SearchManager) listenToStdinAndSearchFiles() {

    lastSearched := ""
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
                if lastSearched != searchManager.searchTerm {
                    searchManager.searchForMatches()
                }
                lastSearched = searchManager.searchTerm
        }
    }
}

func (searchManager *SearchManager) searchForMatches(){
    linesByFilePath := searchManager.fileSearcher.getSearchMatchesByLine(searchManager.searchTerm)
    if len(linesByFilePath) == 0 {
        searchManager.searchState = "NEGATIVE"
    } else {
        searchManager.searchState = "POSITIVE"
    }
    searchManager.renderSearchTerm()
    searchManager.displaySearchMatches(linesByFilePath)
}

func (searchManager *SearchManager) positionCursorAtIndex(){
    fmt.Printf("\033[%d;%dH", searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE, searchManager.cursorIndex+1)
}

func (searchManager *SearchManager) renderSearchTerm(){
    var colorCode string
    if searchManager.searchState == "TYPING" {
        colorCode = "\u001b[34m"
    } else if searchManager.searchState == "POSITIVE" {
        colorCode = "\u001b[32m"
    } else if searchManager.searchState == "NEGATIVE" {
        colorCode = "\u001b[31m"
    } else {
        return //THIS SHOULDN'T HAPPEN
    }
    searchManager.clearTerminalLine(searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE)
    // no need to navigate to TERMINAL_SPACE_SEARCH_TERM_LINE
    // since cursor will be there after clearTerminalLine()
    fmt.Print(colorCode)
    fmt.Print(searchManager.searchTerm)
    fmt.Print("\u001b[0m")
    searchManager.positionCursorAtIndex()
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
    searchManager.positionCursorAtIndex()
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
    searchManager.searchTerm = searchManager.searchTerm[:searchManager.cursorIndex] + char + searchManager.searchTerm[searchManager.cursorIndex:]
    searchManager.incrementCursorIndex()
}

func (searchManager *SearchManager) editSearchTermWithStdin(stdin []byte) {

    if 32 <= stdin[0] && stdin[0] <= 126 { // char is alphanumeric or punctuation
        searchManager.addCharToSearchTerm(string(stdin))
        searchManager.searchState = "TYPING"

    } else if stdin[0] == 4 { // C-d
        if searchManager.cursorIndex < len(searchManager.searchTerm) {
            searchManager.deleteCharForwards()
            searchManager.searchState = "TYPING"
        }

    } else if stdin[0] == 127 { // backspace
        if searchManager.cursorIndex > 0 {
            searchManager.deleteCharBackwards()
            searchManager.decrementCursorIndex()
            searchManager.searchState = "TYPING"
        }

    } else if stdin[0] == 6 { // C-f
        if searchManager.cursorIndex < len(searchManager.searchTerm) {
            searchManager.incrementCursorIndex()
        }

    } else if stdin[0] == 2 { // C-b
        if searchManager.cursorIndex > 0 {
            searchManager.decrementCursorIndex()
        }

    } else {
        return
    }
    searchManager.renderSearchTerm()
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
