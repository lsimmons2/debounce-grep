package main

import (
    "fmt"
    "strings"
    "time"
    "os"
    "os/exec"
    "path/filepath"
    "bufio"
    "strconv"
    "sort"
    "index/suffixarray"
    "regexp"
)


func getTtyWidth() int {
    cmd := exec.Command("tput", "cols")
    cmd.Stdin = os.Stdin
    out, _ := cmd.Output()
    columnCount, _ := strconv.Atoi(string(out)[:2])
    return columnCount - 1 // not sure why this is off by one
}

var (
    ttyWidth = getTtyWidth()
)

const (
    DEBOUNCE_TIME_MS = 300
    SPACE = " "
    //ANSI escape codes to control stdout and cursor in terminal
    MAGENTA_COLOR_CODE = "\u001b[35m"
    RED_COLOR_CODE = "\u001b[31m"
    GREEN_COLOR_CODE = "\u001b[32m"
    BLUE_COLOR_CODE = "\u001b[34m"
    YELLOW_COLOR_CODE = "\u001b[33m"
    CANCEL_COLOR_CODE = "\u001b[0m"
    CLEAR_LINE_CODE = "\033[K"
    NAVIGATE_CURSOR_CODE = "\033[%d;%dH" // passed line and column numbers
    //search term always displayed on this line of terminal
    SEARCH_TERM_TERMINAL_LINE_NO = 2
    //search matches always displayed in space bordered by these lines
    SEARCH_MATCH_SPACE_START_TERMINAL_LINE_NO = 4
    SEARCH_MATCH_SPACE_END_TERMINAL_LINE_NO = 34
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
    fmt.Print(SPACE)
}

func (lineWithMatches *LineWithMatches) renderMatchedLine() {
    lineWithMatches.renderIndent()
    lineWithMatches.renderLineNo()
    var lineToRender string
    for charIndex, char := range lineWithMatches.text {
        nextMatchStartIndex := -1
        nextMatchEndIndex := -1
        if len(lineWithMatches.matchIndeces) > 0 {
            nextMatchIndexPair := lineWithMatches.matchIndeces[0]
            nextMatchStartIndex = nextMatchIndexPair[0]
            nextMatchEndIndex = nextMatchIndexPair[1]
        }
        if charIndex == nextMatchStartIndex {
            lineToRender = lineToRender + string(YELLOW_COLOR_CODE)
        }
        lineToRender = lineToRender + string(char)
        if charIndex == nextMatchEndIndex - 1 {
            lineToRender = lineToRender + string(CANCEL_COLOR_CODE)
            //pop match index pair
            lineWithMatches.matchIndeces = append(lineWithMatches.matchIndeces[:0], lineWithMatches.matchIndeces[1:]...)
        }
    }
    words := strings.Split(lineToRender, SPACE)
    var currentLineLength int
    for _, word := range words {
        lengthOfWord := lineWithMatches.getLengthOfWord(word)
        if lineWithMatches.wordWilllHitEndOfTty(lengthOfWord, currentLineLength) {
            fmt.Println("")
            lineWithMatches.renderIndent()
            lineWithMatches.renderLineNoBufferSpace()
            currentLineLength = (lengthOfWord + len(SPACE))
        } else {
            currentLineLength += (lengthOfWord + len(SPACE))
        }
        fmt.Print(word)
        fmt.Print(SPACE)
    }
    fmt.Println("")
}


func (lineWithMatches *LineWithMatches) getLengthOfWord(word string) int {
    //don't include color codes in length of word
    wordWithoutColorCodes := strings.Replace(word, YELLOW_COLOR_CODE, "", 1)
    wordWithoutColorCodes = strings.Replace(wordWithoutColorCodes, CANCEL_COLOR_CODE, "", 1)
    return len(wordWithoutColorCodes)
}

func (lineWithMatches *LineWithMatches) wordWilllHitEndOfTty(lengthOfWord int, currentLineLength int) bool {
    ttyLength := ttyWidth - 1 - lineWithMatches.INDENT_LENGTH - lineWithMatches.LINE_NO_BUFFER_LENGTH
    return (lengthOfWord + currentLineLength) > ttyLength
}

func (lineWithMatches *LineWithMatches) renderIndent() {
    for i := 1; i <= lineWithMatches.INDENT_LENGTH; i++ { 
        fmt.Print(SPACE)
    }
}

func (lineWithMatches *LineWithMatches) renderLineNoBufferSpace() {
    for i := 1; i <= lineWithMatches.LINE_NO_BUFFER_LENGTH; i++ { 
        fmt.Print(SPACE)
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
        lineWithMatches.renderMatchedLine()
    }
}

type SearchManager struct {
    fileSearcher FileSearcher
    cursorIndex int
    searchTerm string
    searchState string
    selectedMatchIndex int
    filesWithMatches []FileWithMatches
}

func NewSearchManager() *SearchManager {
    searchManager := &SearchManager{}
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
            case <-time.After(time.Duration((1000000 * DEBOUNCE_TIME_MS)) * time.Nanosecond):
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
    fmt.Printf(NAVIGATE_CURSOR_CODE, SEARCH_TERM_TERMINAL_LINE_NO, searchManager.cursorIndex+1)
}

func (searchManager *SearchManager) renderSearchTerm(){
    var colorCode string
    if searchManager.searchState == "TYPING" {
        colorCode = BLUE_COLOR_CODE
    } else if searchManager.searchState == "POSITIVE" {
        colorCode = GREEN_COLOR_CODE
    } else if searchManager.searchState == "NEGATIVE" {
        colorCode = RED_COLOR_CODE
    }
    searchManager.clearTerminalLine(SEARCH_TERM_TERMINAL_LINE_NO)
    // no need to navigate to SEARCH_TERM_TERMINAL_LINE_NO
    // since cursor will be there after clearTerminalLine()
    fmt.Print(colorCode)
    fmt.Print(searchManager.searchTerm)
    fmt.Print(CANCEL_COLOR_CODE)
    searchManager.positionCursorAtIndex()
}

func (searchManager *SearchManager) clearTerminalLine(numberOfLineToClear int){
    COLUMN := 1
    fmt.Printf(NAVIGATE_CURSOR_CODE, numberOfLineToClear, COLUMN)
    fmt.Printf(CLEAR_LINE_CODE)
}

func (searchManager *SearchManager) clearSearchMatchTerminalSpace(){
    for i := SEARCH_MATCH_SPACE_START_TERMINAL_LINE_NO; i <= SEARCH_MATCH_SPACE_END_TERMINAL_LINE_NO; i++ {
        searchManager.clearTerminalLine(i)
    }
    COLUMN := 1
    fmt.Printf(NAVIGATE_CURSOR_CODE, SEARCH_MATCH_SPACE_START_TERMINAL_LINE_NO, COLUMN)
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

    } else if stdin[0] == 0 { // C-space
        searchManager.toggleSelectedMatchShouldShowHits()
        searchManager.displaySearchMatches()

    } else {
        return
    }
    searchManager.renderSearchTerm()
}

func main() {
    searchManager := NewSearchManager()
    searchManager.listenToStdinAndSearchFiles()
}
