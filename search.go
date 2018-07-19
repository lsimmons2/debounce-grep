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
    "sort"
    "index/suffixarray"
    "regexp"
)


func getTtyWidth() int {
    cmd := exec.Command("tput", "cols")
    cmd.Stdin = os.Stdin
    out, _ := cmd.Output()
    cols, _ := strconv.Atoi(string(out)[:2])
    logToFileCursor(cols)
    return cols
}

var (
    ttyWidth = getTtyWidth()
)

const (
    MAGENTA_COLOR_CODE = "\u001b[35m"
    RED_COLOR_CODE = "\u001b[31m"
    GREEN_COLOR_CODE = "\u001b[32m"
    BLUE_COLOR_CODE = "\u001b[34m"
    YELLOW_COLOR_CODE = "\u001b[33m"
    CANCEL_COLOR_CODE = "\u001b[0m"
)

type File struct {
    path string
}

func (file *File) hasShebang() bool{
    for line := range file.fileLinesGenerator(){
        if line == "*study" {
            return true
        }
    }
    return false
}

func (file *File) fileLinesGenerator() <- chan string {
	ch := make(chan string)
	go func() {
        file, err := os.Open(file.path)
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

type LineWithMatches struct {
    lineNo int
    matchIndeces [][]int
    text string
    INDENT_LENGTH int
    LINE_NO_BUFFER_LENGTH int
}

func NewLineWithMatches(lineNo int, matchIndeces [][]int, lineText string) *LineWithMatches {
    lineWithMatches := &LineWithMatches{}
    lineWithMatches.lineNo = lineNo
    lineWithMatches.matchIndeces = matchIndeces
    lineWithMatches.text = lineText
    lineWithMatches.INDENT_LENGTH = 3
    lineWithMatches.LINE_NO_BUFFER_LENGTH = 3
    return lineWithMatches
}

func (lineWithMatches *LineWithMatches) renderLineNo() {
    fmt.Print(lineWithMatches.lineNo)
    fmt.Print(" ")
}

func (lineWithMatches *LineWithMatches) renderLine() {
    lineWithMatches.renderIndent()
    lineWithMatches.renderLineNo()
    for charIndex, char := range lineWithMatches.text {
        nextMatchStartIndex := -1
        nextMatchEndIndex := -1
        if len(lineWithMatches.matchIndeces) > 0 {
            nextMatchIndexPair := lineWithMatches.matchIndeces[0]
            nextMatchStartIndex = nextMatchIndexPair[0]
            nextMatchEndIndex = nextMatchIndexPair[1]
        }
        if charIndex == nextMatchStartIndex {
            fmt.Print(YELLOW_COLOR_CODE)
        }
        fmt.Print(string(char))
        if charIndex == nextMatchEndIndex - 1 {
            fmt.Print(CANCEL_COLOR_CODE)
            lineWithMatches.matchIndeces = append(lineWithMatches.matchIndeces[:0], lineWithMatches.matchIndeces[1:]...)
        }
        if lineWithMatches.isAtLastTtyColumn(charIndex) {
            fmt.Println("")
            lineWithMatches.renderIndent()
            lineWithMatches.renderIndent()
        }
    }
    fmt.Println("")
}

func (lineWithMatches *LineWithMatches) isAtLastTtyColumn(charIndex int) bool {
    if charIndex == 0 {
        return false
    }
    endOfTtyIndex := ttyWidth - 2 - lineWithMatches.INDENT_LENGTH - lineWithMatches.LINE_NO_BUFFER_LENGTH
    return charIndex % endOfTtyIndex == 0
}

func (lineWithMatches *LineWithMatches) renderIndent() {
    for i := 1; i <= lineWithMatches.INDENT_LENGTH; i++ { 
        fmt.Print(" ")
    }
}

func (file *File) getLinesWithMatches(searchTerm string) []LineWithMatches {
    var linesWithMatches []LineWithMatches
    lineNumber := 1
    for line := range file.fileLinesGenerator(){
        if strings.Contains(line, searchTerm) {
            searchTermRegex := regexp.MustCompile(searchTerm)
            index := suffixarray.New([]byte(line))
            matchIndeces := index.FindAllIndex(searchTermRegex, -1)
            lineWithMatches := *NewLineWithMatches(lineNumber, matchIndeces, line)
            linesWithMatches = append(linesWithMatches, lineWithMatches)
        }
        lineNumber ++
    }
    return linesWithMatches
}



type FileSearcher struct {
    filesToSearch []File
}

func NewFileSearcher() *FileSearcher {
    fileSearcher := &FileSearcher{}
    dir := "/home/leo/org"
    err := filepath.Walk(dir, func(path string, info os.FileInfo, _ error) error {
        if info.IsDir() && info.Name() == "venv" || info.Name() == ".git"  {
            return filepath.SkipDir
        }
        file := File{path: path}
        if file.hasShebang() {
            fileSearcher.filesToSearch = append(fileSearcher.filesToSearch, file)
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

func (fileSearcher *FileSearcher) getFilesWithMatches(searchTerm string) []FileWithMatches {
    if len(fileSearcher.filesToSearch) > 0  && len(searchTerm) > 0 {
        var filesWithMatches []FileWithMatches
        for i := 0; i < len(fileSearcher.filesToSearch); i++ {
            var fileWithMatches FileWithMatches
            linesWithMatches := fileSearcher.filesToSearch[i].getLinesWithMatches(searchTerm)
            if len(linesWithMatches) > 0 {
                filePath := fileSearcher.filesToSearch[i].path
                fileWithMatches = *NewFileWithMatches(filePath, linesWithMatches)
                filesWithMatches = append(filesWithMatches, fileWithMatches)
            }
        }
        return filesWithMatches
    }
    return nil
}

type FileWithMatches struct {
    filePath string
    linesWithMatches []LineWithMatches
    isSelected bool
    shouldShowHits bool
}

func NewFileWithMatches(filePath string, linesWithMatches []LineWithMatches) *FileWithMatches {
    fileWithMatches := &FileWithMatches{}
    fileWithMatches.filePath = filePath
    fileWithMatches.linesWithMatches = linesWithMatches
    fileWithMatches.isSelected = false
    fileWithMatches.shouldShowHits = false
    return fileWithMatches
}

func (fileWithMatches *FileWithMatches) render() {
    fileWithMatches.renderFilePath()
    if fileWithMatches.shouldShowHits {
        fileWithMatches.showHits()
    }
}

func (fileWithMatches *FileWithMatches) renderFilePath() {
    if fileWithMatches.isSelected {
        fmt.Print(MAGENTA_COLOR_CODE)
    }
    fmt.Println(fileWithMatches.filePath)
    if fileWithMatches.isSelected {
        fmt.Print(CANCEL_COLOR_CODE)
    }
}

func (fileWithMatches *FileWithMatches) showHits() {
    sort.Slice(fileWithMatches.linesWithMatches, func(i, j int) bool {
        return fileWithMatches.linesWithMatches[i].lineNo < fileWithMatches.linesWithMatches[j].lineNo
    })
    for _, lineWithMatches := range fileWithMatches.linesWithMatches {
        lineWithMatches.renderLine()
    }
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
    selectedMatchIndex int
    filesWithMatches []FileWithMatches
}

func NewSearchManager() *SearchManager {
    searchManager := &SearchManager{}
    searchManager.DEBOUNCE_TIME_MS = 300
    searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE = 2
    searchManager.SEARCH_MATCH_TERMINAL_SPACE_START_LINE = 4
    searchManager.SEARCH_MATCH_TERMINAL_SPACE_END_LINE = 34
    searchManager.cursorIndex = 0
    searchManager.selectedMatchIndex = 0
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
                    searchManager.handleStdinCommands(stdin)
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
    searchManager.filesWithMatches = searchManager.fileSearcher.getFilesWithMatches(searchManager.searchTerm)
    if len(searchManager.filesWithMatches) == 0 {
        searchManager.searchState = "NEGATIVE"
        searchManager.selectedMatchIndex = 0
    } else {
        searchManager.searchState = "POSITIVE"
        searchManager.selectedMatchIndex = 0
    }
    searchManager.renderSearchTerm()
    searchManager.displaySearchMatches()
}

func (searchManager *SearchManager) positionCursorAtIndex(){
    fmt.Printf("\033[%d;%dH", searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE, searchManager.cursorIndex+1)
}

func (searchManager *SearchManager) renderSearchTerm(){
    var colorCode string
    if searchManager.searchState == "TYPING" {
        colorCode = BLUE_COLOR_CODE
    } else if searchManager.searchState == "POSITIVE" {
        colorCode = GREEN_COLOR_CODE
    } else if searchManager.searchState == "NEGATIVE" {
        colorCode = RED_COLOR_CODE
    } else {
        return //THIS SHOULDN'T HAPPEN
    }
    searchManager.clearTerminalLine(searchManager.TERMINAL_SPACE_SEARCH_TERM_LINE)
    // no need to navigate to TERMINAL_SPACE_SEARCH_TERM_LINE
    // since cursor will be there after clearTerminalLine()
    fmt.Print(colorCode)
    fmt.Print(searchManager.searchTerm)
    fmt.Print(CANCEL_COLOR_CODE)
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

func (searchManager *SearchManager) displaySearchMatches(){
    searchManager.clearSearchMatchTerminalSpace()
    if len(searchManager.filesWithMatches) > 0 {
        for index, fileWithMatches := range searchManager.filesWithMatches {
            if index == searchManager.selectedMatchIndex {
                fileWithMatches.isSelected = true
            } else {
                fileWithMatches.isSelected = false
            }
            fileWithMatches.render()
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

func (searchManager *SearchManager) incrementSelectedMatchIndex() {
    searchManager.selectedMatchIndex += 1
}

func (searchManager *SearchManager) decrementSelectedMatchIndex() {
    searchManager.selectedMatchIndex -= 1
}

func (searchManager *SearchManager) toggleSelectedMatchShouldShowHits() {
    searchManager.filesWithMatches[searchManager.selectedMatchIndex].shouldShowHits = !searchManager.filesWithMatches[searchManager.selectedMatchIndex].shouldShowHits
}

func (searchManager *SearchManager) handleStdinCommands(stdin []byte) {

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

    } else if stdin[0] == 10 { // C-j
        if searchManager.selectedMatchIndex < len(searchManager.filesWithMatches) - 1 {
            searchManager.incrementSelectedMatchIndex()
            searchManager.displaySearchMatches()
        }

    } else if stdin[0] == 11 { // C-k
        if searchManager.selectedMatchIndex > 0 {
            searchManager.decrementSelectedMatchIndex()
            searchManager.displaySearchMatches()
        }

    //} else if stdin[0] == 10 { // enter but also C-j
    } else if stdin[0] == 0 { // C-space
        searchManager.toggleSelectedMatchShouldShowHits()
        searchManager.displaySearchMatches()

    //} else if stdin[0] == 5 { // C-e
    //} else if stdin[0] == 1 { // C-a

    } else {
        return
    }
    searchManager.renderSearchTerm()
}

func main() {
    searchManager := NewSearchManager()
    searchManager.listenToStdinAndSearchFiles()
}


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

