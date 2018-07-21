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
    //number of spaces the line numbers of matches are
    //indented from left border of terminal window
    SEARCH_MATCH_SPACE_INDENT = 3
    //number of spaces the text of matches are further
    //indented from SEARCH_MATCH_SPACE_INDENT
    SEARCH_MATCH_SPACE_LINE_NO_BUFFER = 3
    DIR_TO_SEARCH = "/home/leo/org"
)

type File struct {
    path string
    linesWithMatches []LineWithMatches
    isSelected bool
    shouldShowHits bool
}

func NewFile(filePath string, linesWithMatches []LineWithMatches) *File {
    file := &File{}
    file.path = filePath
    file.isSelected = false
    file.shouldShowHits = false
    return file
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

func (file *File) render() {
    file.renderFilePath()
    if file.shouldShowHits {
        file.showHits()
    }
}

func (file *File) renderFilePath() {
    if file.isSelected {
        fmt.Print(MAGENTA_COLOR_CODE)
    }
    fmt.Println(file.path)
    if file.isSelected {
        fmt.Print(CANCEL_COLOR_CODE)
    }
}

func (file *File) showHits() {
    //show lines in increasing order
    sort.Slice(file.linesWithMatches, func(i, j int) bool {
        return file.linesWithMatches[i].lineNo < file.linesWithMatches[j].lineNo
    })
    for _, lineWithMatches := range file.linesWithMatches {
        lineWithMatches.renderMatchedLine()
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



type LineWithMatches struct {
    lineNo int
    matchIndeces [][]int
    text string
}

func NewLineWithMatches(lineNo int, matchIndeces [][]int, lineText string) *LineWithMatches {
    lineWithMatches := &LineWithMatches{}
    lineWithMatches.lineNo = lineNo
    lineWithMatches.matchIndeces = matchIndeces
    lineWithMatches.text = lineText
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
    //insert color code and escape code around each match in line
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
    //print line word by word to ensure that line
    //wrapping doesn't happen in middle of word
    words := strings.Split(lineToRender, SPACE)
    var currentLineLength int
    for _, word := range words {
        lengthOfWord := lineWithMatches.getLengthOfWord(word)
        if lineWithMatches.wordWillHitEndOfTty(lengthOfWord, currentLineLength) {
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

func (lineWithMatches *LineWithMatches) wordWillHitEndOfTty(lengthOfWord int, currentLineLength int) bool {
    ttyLength := ttyWidth - 1 - SEARCH_MATCH_SPACE_INDENT - SEARCH_MATCH_SPACE_LINE_NO_BUFFER
    return (lengthOfWord + currentLineLength) > ttyLength
}

func (lineWithMatches *LineWithMatches) renderIndent() {
    for i := 1; i <= SEARCH_MATCH_SPACE_INDENT; i++ { 
        fmt.Print(SPACE)
    }
}

func (lineWithMatches *LineWithMatches) renderLineNoBufferSpace() {
    for i := 1; i <= SEARCH_MATCH_SPACE_LINE_NO_BUFFER; i++ { 
        fmt.Print(SPACE)
    }
}



type SearchManager struct {
    cursorIndex int
    searchTerm string
    searchState string
    selectedMatchIndex int
    filesToSearch []File
    filesWithMatches []File
}

func NewSearchManager() *SearchManager {
    searchManager := &SearchManager{}
    searchManager.cursorIndex = 0
    searchManager.selectedMatchIndex = 0
    searchManager.searchTerm = ""
    searchManager.searchState = "TYPING"
    searchManager.filesToSearch = searchManager.getFilesToSearch()
    return searchManager
}

func (searchManager *SearchManager) getFilesToSearch() []File{
    var filesToSearch []File
    err := filepath.Walk(DIR_TO_SEARCH, func(path string, info os.FileInfo, _ error) error {
        if info.IsDir() && info.Name() == "venv" || info.Name() == ".git"  {
            return filepath.SkipDir
        }
        file := File{path: path}
        if file.hasShebang() {
            filesToSearch = append(filesToSearch, file)
        }
        return nil
    })
    if err != nil {
        fmt.Printf("error walking the path %q: %v\n", DIR_TO_SEARCH, err)
    }
    return filesToSearch
}

func (searchManager *SearchManager) getFilesWithMatches(searchTerm string) []File {
    if len(searchManager.filesToSearch) > 0  && len(searchTerm) > 0 {
        var filesWithMatches []File
        for i := 0; i < len(searchManager.filesToSearch); i++ {
            searchManager.filesToSearch[i].linesWithMatches = searchManager.filesToSearch[i].getLinesWithMatches(searchTerm)
            if len(searchManager.filesToSearch[i].linesWithMatches) > 0 {
                filesWithMatches = append(filesWithMatches, searchManager.filesToSearch[i])
            }
        }
        return filesWithMatches
    }
    return nil
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
    searchManager.filesWithMatches = searchManager.getFilesWithMatches(searchManager.searchTerm)
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
