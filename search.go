package main

import (
    "fmt"
    "strings"
    "time"
    "os"
    "os/exec"
    "os/user"
    "path/filepath"
    "bufio"
    "strconv"
    "sort"
    "index/suffixarray"
    "regexp"
    "github.com/mattn/go-zglob"
    "log"
    "math"
    "github.com/maxmclau/gput"
)

func printNewLine() {
    fmt.Println("")
}

//this can be done with math.Round in go 1.10
func round(x float64) int {
    t := math.Trunc(x)
    if math.Abs(x-t) >= 0.5 {
        return int(t + math.Copysign(1, x))
    }
    return int(t)
}

func setUpLogging() int{
    f, err := os.OpenFile("log.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
    if err != nil {
        log.Fatalf("error opening file: %v", err)
    }
    //defer f.Close()
    log.SetOutput(f)
    return 1
}

func getTtyDimensions() (int, int) {
    lines := gput.Lines()
    cols := gput.Cols()
    log.Printf("Detected tty dimensions: %v x %v.", lines, cols)
    return lines, cols
}

func getDirToSearch() string {
    dirToSearchEnvVariable := os.Getenv("DEBOUNCE_GREP_DIR_TO_SEARCH")
    //check if dir exists
    _, err := os.Stat(dirToSearchEnvVariable)
    if err == nil {
        return dirToSearchEnvVariable
    }
    usr, _ := user.Current()
    return usr.HomeDir
}

func getDebounceTimeMS() int {
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

func getFileShebangs() []string {
    return getEnvVariableList("DEBOUNCE_GREP_FILE_SHEBANG")
}

func getFullPathsToIgnore() []string {
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

func getDirsToSearch() []string {
    var dirsToSearchFromEnv []string
    dirsToSearchFromEnv = getEnvVariableList("DEBOUNCE_GREP_FILES_DIRS_TO_SEARCH")
    if len(dirsToSearchFromEnv) == 0 {
        cwd, _ := filepath.Abs(filepath.Dir(os.Args[0]))
        dirsToSearchFromEnv = append(dirsToSearchFromEnv, cwd)
    }
    return dirsToSearchFromEnv
}

var (
    //only declaring this var so that logging is initialized before
    //other variables are declared
    sah = setUpLogging()
    ttyHeight, ttyWidth = getTtyDimensions()
    debounceTimeMs = getDebounceTimeMS()
    //type of shebang or mark that user can specify to only
    //include files that contain shebang
    fileShebangs = getFileShebangs()
    dirsToSearch = getDirsToSearch()
    fullPathsToIgnore = getFullPathsToIgnore()
    shouldTruncateMatchedLines = true
    maxLinesToPrintPerFile = 5
)

const (
    SPACE = " "
    LINE_BREAK = "\n"
    ELLIPSIS = "..."
    //ANSI escape codes to control stdout and cursor in terminal
    MAGENTA_COLOR_CODE = "\u001b[35m"
    RED_COLOR_CODE = "\u001b[31m"
    GREEN_COLOR_CODE = "\u001b[32m"
    GREEN_BACKGROUND_COLOR_CODE = "\u001b[42m"
    BLUE_COLOR_CODE = "\u001b[34m"
    YELLOW_COLOR_CODE = "\u001b[33m"
    CANCEL_COLOR_CODE = "\u001b[0m"
    CLEAR_LINE_CODE = "\033[K"
    NAVIGATE_CURSOR_CODE = "\033[%d;%dH" // passed line and column numbers
    //search term always rendered on this line of terminal
    SEARCH_TERM_TERMINAL_LINE_NO = 1
    //search matches always rendered in space bordered by these lines
    SEARCH_MATCH_SPACE_START_TERMINAL_LINE_NO = 2
    //indent between line numbers of matches and left border of tty
    SEARCH_MATCH_SPACE_INDENT = "   "
    //indent between text of matches and where line numbers of matches start
    LINE_NO_BUFFER = "   "
    SCROLL_BAR_WIDTH = 1
)



type File struct {
    path string
    linesWithMatches []LineWithMatches
    isSelected bool
    shouldShowMatches bool
}

func NewFile(filePath string, linesWithMatches []LineWithMatches) *File {
    file := &File{}
    file.path = filePath
    file.isSelected = false
    file.shouldShowMatches = false
    return file
}

func (file *File) hasShebang() bool{
    if fileShebangs == nil {
        return true
    }
    for line := range file.fileLinesGenerator(){
        for _, shebang := range fileShebangs {
            if line == shebang {
                return true
            }
        }
    }
    return false
}

func (file *File) fileLinesGenerator() <- chan string {
	ch := make(chan string)
	go func() {
        file, _ := os.Open(file.path)
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
    if file.shouldShowMatches {
        file.showMatches()
    }
}

func (file *File) renderFilePath() {
    if file.isSelected {
        fmt.Print(MAGENTA_COLOR_CODE)
    }
    var numberOfMatchesInFile int
    for _, lineWithMatches := range file.linesWithMatches {
        numberOfMatchesInFile += len(lineWithMatches.matchIndeces)
    }

    matchesString := "matches"
    if numberOfMatchesInFile == 1 {
        matchesString = "match"
    }

    linesString := "lines"
    if len(file.linesWithMatches) == 1 {
        linesString = "line"
    }

    if file.shouldShowMatches {
        fmt.Printf("%v - %v %v on %v %v", file.path, numberOfMatchesInFile, matchesString, len(file.linesWithMatches), linesString)
    } else {
        fmt.Printf("%v - %v %v", file.path, numberOfMatchesInFile, matchesString)
    }
    if file.isSelected {
        fmt.Print(CANCEL_COLOR_CODE)
    }
}

func (file *File) showMatches() {
    //show lines in increasing order
    sort.Slice(file.linesWithMatches, func(i, j int) bool {
        return file.linesWithMatches[i].lineNo < file.linesWithMatches[j].lineNo
    })
    printNewLine()
    numberOfLinesPrinted := 0
    for _, lineWithMatches := range file.linesWithMatches {
        lineWithMatches.renderMatchedLine()
        numberOfLinesPrinted += 1
        if numberOfLinesPrinted == maxLinesToPrintPerFile {
            return
        }
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

func (lineWithMatches *LineWithMatches) popNextMatchIndeces() (int, int) {
    nextMatchIndexPair := lineWithMatches.matchIndeces[0]
    nextMatchStartIndex := nextMatchIndexPair[0]
    nextMatchEndIndex := nextMatchIndexPair[1]
    lineWithMatches.matchIndeces = append(lineWithMatches.matchIndeces[:0], lineWithMatches.matchIndeces[1:]...)
    return nextMatchStartIndex, nextMatchEndIndex
}

func (lineWithMatches *LineWithMatches) getWordsWithColorCodes() []string {
    //insert color code and escape code around each match in line
    var lineToRender string
    nextMatchStartIndex, nextMatchEndIndex := lineWithMatches.popNextMatchIndeces()
    for charIndex, char := range lineWithMatches.text {
        if charIndex == nextMatchStartIndex {
            lineToRender = lineToRender + string(YELLOW_COLOR_CODE)
        }
        lineToRender = lineToRender + string(char)
        if charIndex == nextMatchEndIndex - 1 {
            lineToRender = lineToRender + string(CANCEL_COLOR_CODE)
            if len(lineWithMatches.matchIndeces) > 0 {
                nextMatchStartIndex, nextMatchEndIndex = lineWithMatches.popNextMatchIndeces()
            } else {
                nextMatchStartIndex = -1
                nextMatchEndIndex = -1
            }
        }
    }
    //print line word by word to ensure that line
    //wrapping doesn't happen in middle of word
    words := strings.Split(lineToRender, SPACE)
    if shouldTruncateMatchedLines {
        var firstMatchedWordIndex int
        for wordIndex, word := range words {
            if strings.Contains(word, CANCEL_COLOR_CODE) {
                firstMatchedWordIndex = wordIndex
                break
            }
        }
        var firstWordToShowIndex int
        if firstMatchedWordIndex < 4 {
            firstWordToShowIndex = 0
        } else {
            firstWordToShowIndex = firstMatchedWordIndex - 3
        }
        return words[firstWordToShowIndex:]
    }
    return words
}

func (lineWithMatches *LineWithMatches) renderMatchedLine() {
    fmt.Print(SEARCH_MATCH_SPACE_INDENT)
    fmt.Print(lineWithMatches.lineNo)
    fmt.Print(SPACE)
    lineWithMatches.renderMatchedLineText()
    printNewLine()
}

func (lineWithMatches *LineWithMatches) renderMatchedLineText() {
    words := lineWithMatches.getWordsWithColorCodes()
    var entitiesToPrint []string
    for _, word := range words {
        if lineWithMatches.entityWillHitEndOfTty(word, entitiesToPrint) {
            log.Printf("Entity \"%v\" will hit end of line.", word)
            if shouldTruncateMatchedLines {
                //make sure ellipsis doesn't hit end of tty
                for lineWithMatches.entityWillHitEndOfTty(ELLIPSIS, entitiesToPrint) {
                    entitiesToPrint = entitiesToPrint[:len(entitiesToPrint)-1]
                }
                //make sure last entity before ellipsis isn't space
                if entitiesToPrint[len(entitiesToPrint)-1] == SPACE {
                    entitiesToPrint = entitiesToPrint[:len(entitiesToPrint)-1]
                }
                entitiesToPrint = append(entitiesToPrint, ELLIPSIS)
                break
            } else {
                entitiesToPrint = append(entitiesToPrint, LINE_BREAK)
                entitiesToPrint = append(entitiesToPrint, SEARCH_MATCH_SPACE_INDENT)
                entitiesToPrint = append(entitiesToPrint, LINE_NO_BUFFER)
            }
        }
        log.Printf("Entity \"%v\" will not hit end of line.", word)
        entitiesToPrint = append(entitiesToPrint, word)
        entitiesToPrint = append(entitiesToPrint, SPACE)
    }
    for _, entity := range entitiesToPrint {
        fmt.Print(entity)
    }
}

func (lineWithMatches *LineWithMatches) entityWillHitEndOfTty(entity string, entitiesToPrint []string) bool {
    roomForText := ttyWidth - 1 - len(SEARCH_MATCH_SPACE_INDENT) - len(LINE_NO_BUFFER) - SCROLL_BAR_WIDTH
    lineLength := lineWithMatches.getLengthOfEntity(entity)
    for _, entity := range entitiesToPrint {
        lineLength += lineWithMatches.getLengthOfEntity(entity)
    }
    return lineLength > roomForText
}

func (lineWithMatches *LineWithMatches) getLengthOfEntity(entity string) int {
    //don't include color codes in length of words
    wordWithoutColorCodes := strings.Replace(entity, YELLOW_COLOR_CODE, "", 1)
    wordWithoutColorCodes = strings.Replace(wordWithoutColorCodes, CANCEL_COLOR_CODE, "", 1)
    lengthOfEntity := len(wordWithoutColorCodes)
    log.Printf("Calculated length of \"%v\" to be %v (w/o color codes).", wordWithoutColorCodes, lengthOfEntity)
    return lengthOfEntity
}




type SearchManager struct {
    cursorIndex int
    searchTerm string
    searchState string
    selectedMatchIndex int
    filesToSearch []File
    filesWithMatches []File
    searchingMessageLastPrinted string
    timeLastPrintedSearchMessage int64
    matchIndexAtTopOfWindow int
    cursorLineNo int
}

func NewSearchManager() *SearchManager {
    searchManager := &SearchManager{}
    searchManager.cursorIndex = 0
    searchManager.selectedMatchIndex = 0
    searchManager.searchTerm = ""
    searchManager.searchState = "TYPING"
    searchManager.filesToSearch = searchManager.getFilesToSearch()
    searchManager.searchingMessageLastPrinted = ""
    searchManager.timeLastPrintedSearchMessage = time.Now().UnixNano()
    return searchManager
}

func (searchManager *SearchManager) printSearchingMessage(searchMessageTemplate string) {
    if time.Now().UnixNano() - searchManager.timeLastPrintedSearchMessage >= 150000000 {
        var searchMessage string
        if searchManager.searchingMessageLastPrinted == "" {
            searchMessage = searchMessageTemplate
        } else if searchManager.searchingMessageLastPrinted == searchMessageTemplate {
            searchMessage = searchMessageTemplate + "."
        } else if searchManager.searchingMessageLastPrinted == searchMessageTemplate + "." {
            searchMessage = searchMessageTemplate + ".."
        } else if searchManager.searchingMessageLastPrinted == searchMessageTemplate + ".." {
            searchMessage = searchMessageTemplate + "..."
        } else if searchManager.searchingMessageLastPrinted == searchMessageTemplate + "..." {
            searchMessage = searchMessageTemplate
        }
        searchManager.printAtSearchTermLine(searchMessage)
        searchManager.searchingMessageLastPrinted = searchMessage
        searchManager.timeLastPrintedSearchMessage = time.Now().UnixNano()
    }
}

func (searchManager *SearchManager) getFilesToSearch() []File {
    var skipped []string
    var filesToSearch []File
    for _, dirToSearch := range dirsToSearch {
        err := filepath.Walk(dirToSearch, func(path string, info os.FileInfo, _ error) error {
            shouldSkipFile := false
            searchManager.printSearchingMessage("Gathering files to search")
            for _, pathToIgnore := range fullPathsToIgnore {
                if pathToIgnore == path {
                    skipped = append(skipped, path)
                    if info.IsDir() {
                        return filepath.SkipDir
                    } else {
                        shouldSkipFile = true
                    }
                }
            }
            file := File{path: path}
            if file.hasShebang() && !shouldSkipFile {
                filesToSearch = append(filesToSearch, file)
            }
            return nil
        })
        if err != nil {
            fmt.Printf("error walking the path %q: %v\n", dirToSearch, err)
        }
    }
    searchManager.printAtSearchTermLine("Ready To Search")
    return filesToSearch
}

func (searchManager *SearchManager) printAtSearchTermLine(toPrint string) {
    searchManager.clearSearchMatchTerminalSpace()
    fmt.Print(toPrint)
}

func (searchManager *SearchManager) getFilesWithMatches(searchTerm string) []File {
    if len(searchManager.filesToSearch) > 0  && len(searchTerm) > 0 {
        var filesWithMatches []File
        for i := 0; i < len(searchManager.filesToSearch); i++ {
            searchManager.printSearchingMessage("Searching files")
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
            //debounceTimeMs has passed w/o any stdin
            case <-time.After(time.Duration((1000000 * debounceTimeMs)) * time.Nanosecond):
                if lastSearched != searchManager.searchTerm {
                    searchManager.searchForMatches()
                }
                lastSearched = searchManager.searchTerm
        }
    }
}

func (searchManager *SearchManager) searchForMatches(){
    searchManager.filesWithMatches = searchManager.getFilesWithMatches(searchManager.searchTerm)
    log.Printf("%v matches found.", len(searchManager.filesWithMatches))
    if len(searchManager.filesWithMatches) == 0 {
        searchManager.searchState = "NEGATIVE"
        searchManager.selectedMatchIndex = 0
    } else {
        searchManager.searchState = "POSITIVE"
        searchManager.selectedMatchIndex = 0
    }
    searchManager.matchIndexAtTopOfWindow = 0
    searchManager.cursorLineNo = 2
    searchManager.renderSearchTerm()
    searchManager.renderSearchMatches()
    searchManager.renderScrollBar()
}

func (searchManager *SearchManager) positionCursorAtIndex(){
    log.Printf("Positioning cursor at index at %vx%v.", SEARCH_TERM_TERMINAL_LINE_NO, searchManager.cursorIndex+1)
    searchManager.navigateToLineAndColumn(SEARCH_TERM_TERMINAL_LINE_NO, searchManager.cursorIndex+1)
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

func (searchManager *SearchManager) navigateToLineAndColumn(line int, column int){
    fmt.Printf(NAVIGATE_CURSOR_CODE, line, column)
}

func (searchManager *SearchManager) clearTerminalLine(numberOfLineToClear int){
    searchManager.navigateToLineAndColumn(numberOfLineToClear, 1)
    fmt.Printf(CLEAR_LINE_CODE)
}

func (searchManager *SearchManager) clearSearchMatchTerminalSpace(){
    log.Printf("Clearing terminal search space.")
    for i := SEARCH_MATCH_SPACE_START_TERMINAL_LINE_NO; i <= ttyHeight; i++ {
        searchManager.clearTerminalLine(i)
    }
    searchManager.navigateToLineAndColumn(SEARCH_MATCH_SPACE_START_TERMINAL_LINE_NO, 1)
}

func (searchManager *SearchManager) renderSearchMatches(){
    searchManager.clearSearchMatchTerminalSpace()
    searchManager.navigateToLineAndColumn(1, 1)
    if len(searchManager.filesWithMatches) > 0 {
        for index := searchManager.matchIndexAtTopOfWindow; index <= searchManager.matchIndexAtTopOfWindow + ttyHeight - 2; index++ {
            if len(searchManager.filesWithMatches) <= index {
                break
            }
            fileWithMatches := searchManager.filesWithMatches[index]
            if index == searchManager.selectedMatchIndex {
                fileWithMatches.isSelected = true
            } else {
                fileWithMatches.isSelected = false
            }
            printNewLine()
            fileWithMatches.render()
        }
    }
    searchManager.positionCursorAtIndex()
}

func (searchManager *SearchManager) renderScrollBar(){
    if len(searchManager.filesWithMatches) < ttyHeight {
        log.Printf("100%% of matches shown in tty window, not rendering scroll bar.")
        return
    }
    percentageMatchesShown := float64(ttyHeight) / float64(len(searchManager.filesWithMatches))
    heightOfScrollBar := round(percentageMatchesShown * float64(ttyHeight))
    log.Printf("Calculated scroll bar height to be %v lines (%.2f%% of tty height %v).", heightOfScrollBar, percentageMatchesShown, ttyHeight)
    scrollBarStartLine := int((float64(searchManager.matchIndexAtTopOfWindow) / float64(len(searchManager.filesWithMatches))) * float64(ttyHeight))
    log.Printf("Caclulated scroll bar to start from %v.", scrollBarStartLine)
    for i := scrollBarStartLine + 1; i <= scrollBarStartLine + heightOfScrollBar; i++ {
        searchManager.navigateToLineAndColumn(i, ttyWidth)
        fmt.Printf(GREEN_BACKGROUND_COLOR_CODE)
        for i := 0; i < SCROLL_BAR_WIDTH; i++ {
            fmt.Printf(" ")
        }
        fmt.Printf(CANCEL_COLOR_CODE)
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
    log.Printf("searchManager.selectedMatchIndex incremented to %v", searchManager.selectedMatchIndex)
    log.Printf("cursorLineNo  %v and ttyHeight %v", searchManager.cursorLineNo , ttyHeight)
    if searchManager.cursorLineNo == ttyHeight {
        searchManager.matchIndexAtTopOfWindow += 1
        log.Printf("searchManager.matchIndexAtTopOfWindow incremented to %v", searchManager.matchIndexAtTopOfWindow)
    } else {
        searchManager.cursorLineNo += 1
        log.Printf("searchManager.cursorLineNo incremented to %v", searchManager.cursorLineNo)
    }
}

func (searchManager *SearchManager) decrementSelectedMatchIndex() {
    searchManager.selectedMatchIndex -= 1
    log.Printf("searchManager.selectedMatchIndex decremented to  %v", searchManager.selectedMatchIndex)
    log.Printf("cursorLineNo  %v and ttyHeight %v", searchManager.cursorLineNo , ttyHeight)
    if searchManager.cursorLineNo == 2 {
        searchManager.matchIndexAtTopOfWindow -= 1
        log.Printf("DECREMENTING matchIndexAtTopOfWindow now at %v", searchManager.matchIndexAtTopOfWindow)
    } else {
        searchManager.cursorLineNo -= 1
    }
}

func (searchManager *SearchManager) toggleSelectedMatchShouldShowMatches() {
    searchManager.filesWithMatches[searchManager.selectedMatchIndex].shouldShowMatches = !searchManager.filesWithMatches[searchManager.selectedMatchIndex].shouldShowMatches
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
            searchManager.renderSearchMatches()
        }

    } else if stdin[0] == 11 { // C-k
        if searchManager.selectedMatchIndex > 0 {
            searchManager.decrementSelectedMatchIndex()
            searchManager.renderSearchMatches()
        }

    } else if stdin[0] == 0 { // C-space
        searchManager.toggleSelectedMatchShouldShowMatches()
        searchManager.renderSearchMatches()

    } else {
        //not chars being added to search term or a recognized command
        return
    }
    searchManager.renderSearchTerm()
    searchManager.renderScrollBar()
}

func main() {
    log.Printf("Starting program.\n\n\n")
    searchManager := NewSearchManager()
    searchManager.listenToStdinAndSearchFiles()
}
