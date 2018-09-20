package main

import (
    "fmt"
    "strings"
    "time"
    "os"
    "os/exec"
    "path/filepath"
    "bufio"
    "sort"
    "index/suffixarray"
    "regexp"
    "log"
    "github.com/mattn/go-zglob"
    ut "debounce_grep/utilities"
    "debounce_grep/config"
)


var (
    ttyHeight, ttyWidth = ut.GetTtyDimensions()
    //see config package for description of config options
    Config = config.Values
    debounceTimeMs = Config["debounceTimeMs"].(int)
    maxLinesToPrintPerFile = Config["maxLinesToPrintPerFile"].(int)
    dirsToSearch = Config["dirsToSearch"].([]string)
    fileShebangs = Config["fileShebangs"].([]string)
    patternsToIgnore = Config["patternsToIgnore"].([]string)
    shouldPrintWholeLines = Config["shouldPrintWholeLines"].(bool)
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
    isOpen bool
}

func NewFile(filePath string, linesWithMatches []LineWithMatches) *File {
    file := &File{}
    file.path = filePath
    file.isSelected = false
    file.isOpen = false
    return file
}

func (file *File) hasShebang() bool{
    if len(fileShebangs) == 0 {
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
    if file.isOpen {
        file.open()
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

    if file.isOpen {
        fmt.Printf("%v - %v %v on %v %v", file.path, numberOfMatchesInFile, matchesString, len(file.linesWithMatches), linesString)
    } else {
        fmt.Printf("%v - %v %v", file.path, numberOfMatchesInFile, matchesString)
    }
    if file.isSelected {
        fmt.Print(CANCEL_COLOR_CODE)
    }
}

func (file *File) open() {
    //show matched lines in increasing order
    sort.Slice(file.linesWithMatches, func(i, j int) bool {
        return file.linesWithMatches[i].lineNo < file.linesWithMatches[j].lineNo
    })
    ut.PrintNewLine()
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

func (file *File) getNumberOfLinesRendered() int {
    //2: one line for file path, one for space under matches
    if len(file.linesWithMatches) > maxLinesToPrintPerFile {
        return maxLinesToPrintPerFile + 2
    }
    return len(file.linesWithMatches) + 2
}


type LineWithMatches struct {
    lineNo int
    matchIndeces [][]int
    text string
    wordsWithColorCodes []string
}

func NewLineWithMatches(lineNo int, matchIndeces [][]int, lineText string) *LineWithMatches {
    lineWithMatches := &LineWithMatches{}
    lineWithMatches.lineNo = lineNo
    lineWithMatches.matchIndeces = matchIndeces
    lineWithMatches.text = lineText
    return lineWithMatches
}

func (lineWithMatches *LineWithMatches) getMatchIndeces(indexOfIndeces int) (int, int) {
    //not popping here since I don't want to mutate the state of matchIndeces since
    //this method is called in more than one call of getWordsWithColorCodes
    nextMatchIndexPair := lineWithMatches.matchIndeces[indexOfIndeces]
    nextMatchStartIndex, nextMatchEndIndex := nextMatchIndexPair[0], nextMatchIndexPair[1]
    return nextMatchStartIndex, nextMatchEndIndex
}

func (lineWithMatches *LineWithMatches) getWordsWithColorCodes() []string {
    //insert color code and escape code around each match in line
    var lineToRender string
    nextMatchIndexPairIndex := 0 //index of a pair of indeces
    nextMatchStartIndex, nextMatchEndIndex := lineWithMatches.getMatchIndeces(nextMatchIndexPairIndex)
    for charIndex, char := range lineWithMatches.text {
        if charIndex == nextMatchStartIndex {
            lineToRender = lineToRender + string(YELLOW_COLOR_CODE)
        }
        lineToRender = lineToRender + string(char)
        if charIndex == nextMatchEndIndex - 1 {
            lineToRender = lineToRender + string(CANCEL_COLOR_CODE)
            if nextMatchIndexPairIndex < len(lineWithMatches.matchIndeces) - 1 {
                nextMatchIndexPairIndex ++
                nextMatchStartIndex, nextMatchEndIndex = lineWithMatches.getMatchIndeces(nextMatchIndexPairIndex)
            } else {
                nextMatchStartIndex = -1
                nextMatchEndIndex = -1
            }
        }
    }
    words := strings.Split(lineToRender, SPACE)
    log.Printf("Returning words with color codes: \"%v\"", words)
    return words
}

func (lineWithMatches *LineWithMatches) lineWillHitEndOfTty() {

}

func (lineWithMatches *LineWithMatches) renderMatchedLine() {
    fmt.Print(SEARCH_MATCH_SPACE_INDENT)
    fmt.Print(lineWithMatches.lineNo)
    fmt.Print(SPACE)
    lineWithMatches.renderMatchedLineText()
    ut.PrintNewLine()
}

func (lineWithMatches *LineWithMatches) removeSpacesOnEnds(entitiesToPrint []string) []string {
    if entitiesToPrint[0] == SPACE {
        entitiesToPrint = entitiesToPrint[1:]
    }
    if entitiesToPrint[len(entitiesToPrint)-1] == SPACE {
        entitiesToPrint = entitiesToPrint[:len(entitiesToPrint)-1]
    }
    return entitiesToPrint
}

func (lineWithMatches *LineWithMatches) renderMatchedLineText() {
    var entitiesToPrint []string
    words := lineWithMatches.getWordsWithColorCodes()
    if !shouldPrintWholeLines {
        wholeLine := make([]string, 0)
        for _, word := range words {
            wholeLine = append(wholeLine, word)
            wholeLine = append(wholeLine, SPACE)
        }
        wholeLine = wholeLine[:len(wholeLine)-1]
        if !lineWithMatches.entityWillHitEndOfTty("", wholeLine) {
            entitiesToPrint = wholeLine
            log.Printf("Line \"%v\" will fit in Tty.", wholeLine)
        } else {
            log.Printf("Line \"%v\" will not fit in Tty, truncating line.", wholeLine)
            entitiesToPrint = lineWithMatches.getTruncatedLine(words)
        }
    } else {
        //if not truncating lines, just add line break
        //and buffer spaces whenever text hits end of tty
        entitiesToPrint = lineWithMatches.insertLineBreaksAndBuffers(words)
    }
    for _, entity := range entitiesToPrint {
        fmt.Print(entity)
    }
}

func (lineWithMatches *LineWithMatches) getTruncatedLine(words []string) []string {
    //make sure first match will be in line with at most three words after it
    var firstMatchedWordIndex int
    for wordIndex, word := range words {
        if strings.Contains(word, CANCEL_COLOR_CODE) {
            log.Printf("First match in line found to be index %v: \"%v\".", wordIndex, word)
            firstMatchedWordIndex = wordIndex
            break
        }
    }
    firstMatchedWord := words[firstMatchedWordIndex]
    //if first matched word hits end of tty truncate it and return it with ellipsis
    if lineWithMatches.entityWillHitEndOfTty(firstMatchedWord, []string{ELLIPSIS}){
        roomForText := ttyWidth - 1 - len(SEARCH_MATCH_SPACE_INDENT) - len(LINE_NO_BUFFER) - SCROLL_BAR_WIDTH
        lengthOfTruncatedEntity := roomForText-len(ELLIPSIS) + len(YELLOW_COLOR_CODE) + len(CANCEL_COLOR_CODE)
        singleTruncatedEntity := firstMatchedWord[:lengthOfTruncatedEntity]
        return []string{singleTruncatedEntity, ELLIPSIS}
    }
    entitiesToPrint := []string{firstMatchedWord, ELLIPSIS}
    leftOfFirstMatchIndex := firstMatchedWordIndex - 1
    rightOfFirstMatchIndex := firstMatchedWordIndex + 1
    leftsTurn := true // else its right turn
    needsSpaceLeft := true
    needsSpaceRight := true
    var entityToAdd string
    keepAddingToLine := true
    for keepAddingToLine {
        if leftsTurn {
            if leftOfFirstMatchIndex < 0 {
                leftsTurn = false
                continue
            }
            if needsSpaceLeft {
                entityToAdd = SPACE
                needsSpaceLeft = false
            } else {
                entityToAdd = words[leftOfFirstMatchIndex]
                needsSpaceLeft = true
                leftOfFirstMatchIndex --
            }
        } else {
            if rightOfFirstMatchIndex > len(words) - 1 {
                leftsTurn = true
                continue
            }
            if needsSpaceRight {
                entityToAdd = SPACE
                needsSpaceRight = false
            } else {
                entityToAdd = words[rightOfFirstMatchIndex]
                needsSpaceRight = true
                rightOfFirstMatchIndex ++
            }
        }

        if lineWithMatches.entityWillHitEndOfTty(entityToAdd, entitiesToPrint){
            log.Printf("Entity \"%v\" will hit end of tty, ending line.", entityToAdd)
            keepAddingToLine = false
            break
        }
        if leftsTurn {
            entitiesToPrint = append([]string{entityToAdd}, entitiesToPrint...)
            leftsTurn = false
        } else {
            //add element to the left of the last element which is ELLIPSIS
            entitiesToPrint = append(entitiesToPrint[:len(entitiesToPrint)-1], entityToAdd)
            entitiesToPrint = append(entitiesToPrint, ELLIPSIS)
            leftsTurn = true
        }
    }
    //if there's a space before ellipsis, remove it
    secondToLastIndex := len(entitiesToPrint)-2
    if entitiesToPrint[secondToLastIndex] == SPACE {
        entitiesToPrint = append(entitiesToPrint[:secondToLastIndex], entitiesToPrint[secondToLastIndex+1:]...)
    }
    entitiesToPrint = lineWithMatches.removeSpacesOnEnds(entitiesToPrint)
    return entitiesToPrint
}

func (lineWithMatches *LineWithMatches) insertLineBreaksAndBuffers(words []string) []string {
    var entitiesToPrint = make([]string, 0)
    for _, word := range words {
        if lineWithMatches.entityWillHitEndOfTty(word, entitiesToPrint) {
            log.Printf("Entity \"%v\" will hit end of line.", word)
            entitiesToPrint = lineWithMatches.removeSpacesOnEnds(entitiesToPrint)
            entitiesToPrint = append(entitiesToPrint, LINE_BREAK)
            entitiesToPrint = append(entitiesToPrint, SEARCH_MATCH_SPACE_INDENT)
            entitiesToPrint = append(entitiesToPrint, LINE_NO_BUFFER)
        }
        entitiesToPrint = append(entitiesToPrint, word)
        entitiesToPrint = append(entitiesToPrint, SPACE)
    }
    return entitiesToPrint
}

func (lineWithMatches *LineWithMatches) entityWillHitEndOfTty(entity string, entitiesToPrint []string) bool {
    //entity being a word, a space, or an ellipsis
    if len(entitiesToPrint) == 0 {
        return false
    }
    //TODO: what is this 1 from? - make it explicit here
    roomForText := ttyWidth - 1 - len(SEARCH_MATCH_SPACE_INDENT) - len(LINE_NO_BUFFER) - SCROLL_BAR_WIDTH
    //only check words since last LINE_BREAK for lineLength
    lastLineBreakIndex := -1
    for i := len(entitiesToPrint)-1; i >= 0; i-- {
        if entitiesToPrint[i] == LINE_BREAK {
            lastLineBreakIndex = i
            break
        }
    }
    lineLength := lineWithMatches.getLengthOfEntity(entity)
    for _, entity := range entitiesToPrint[lastLineBreakIndex+1:] {
        lineLength += lineWithMatches.getLengthOfEntity(entity)
    }
    return lineLength > roomForText
}

func (lineWithMatches *LineWithMatches) getLengthOfEntity(entity string) int {
    //don't include color codes in length of words
    wordWithoutColorCodes := strings.Replace(entity, YELLOW_COLOR_CODE, "", 1)
    wordWithoutColorCodes = strings.Replace(wordWithoutColorCodes, CANCEL_COLOR_CODE, "", 1)
    lengthOfEntity := len(wordWithoutColorCodes)
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
    openFileIndexQueue []int
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
    searchManager.openFileIndexQueue = make([]int, 0)
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
    var toIgnore []string
    var filesToSearch []File
    for _, dirToSearch := range dirsToSearch {
        searchingMessage := fmt.Sprintf("Finding files to search in %v", dirToSearch)
        err := filepath.Walk(dirToSearch, func(path string, info os.FileInfo, _ error) error {
            searchManager.printSearchingMessage(searchingMessage)
            indexToRemove := -1
            for toIgnoreIndex, toIgnorePath := range toIgnore {
                if toIgnorePath == path {
                    indexToRemove = toIgnoreIndex
                    break
                }
            }
            if indexToRemove > -1 {
                //remove from toIgnore
                toIgnore = append(toIgnore[:indexToRemove], toIgnore[indexToRemove+1:]...)
                //skip dirs to ignore
                if info.IsDir() {
                    return filepath.SkipDir
                }
                //don't do anything with file to ignore
            } else {
                //search dir for dirs/files to ignore
                if info.IsDir() {
                    for _, patternToIgnore := range patternsToIgnore {
                        toIgnoreMatches, _ := zglob.Glob(path + "/" + patternToIgnore)
                        toIgnore = append(toIgnore, toIgnoreMatches...)
                    }
                //check file for shebang and add accordingly
                } else {
                    file := File{path: path}
                    if file.hasShebang() {
                        filesToSearch = append(filesToSearch, file)
                    }
                }

            }
            return nil
        })
        if err != nil {
            fmt.Printf("error walking the path %q: %v\n", dirToSearch, err)
        }
    }
    log.Printf("Retrieved %v files to search.", len(filesToSearch))
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

        filesInWindowIndeces := make([]int, 0)
        linesTakenUpByOpenFiles := 0
        linesToSpareForMatches := ttyHeight - 1

        //1) FIRST LOOP THROUGH OPENED FILES HITTING MOST RECENTLY OPENED FIRST
        for i := len(searchManager.openFileIndexQueue)-1; i >= 0; i-- {
            openFileIndex := searchManager.openFileIndexQueue[i]
            openFile := searchManager.filesWithMatches[openFileIndex]
            linesForFile := openFile.getNumberOfLinesRendered()
            //if there's room for file
            if linesToSpareForMatches - linesForFile >= 0 {
                filesInWindowIndeces = append(filesInWindowIndeces, openFileIndex)
                linesToSpareForMatches -= linesForFile
                linesTakenUpByOpenFiles += linesForFile
            } else {
                //if only one file is open, then print it even though its
                //hitting bottom of tty - user's combo of
                //maxLinesToPrintPerFile and shouldPrintWholeLines
                //is unfeasible
                if len(searchManager.openFileIndexQueue) == 1 {
                    filesInWindowIndeces = append(filesInWindowIndeces, openFileIndex)
                    linesToSpareForMatches -= linesForFile
                    linesTakenUpByOpenFiles += linesForFile
                }
            }
        }
        log.Printf("Open files taking up %v lines of tty space, %v lines left for closed files.", linesTakenUpByOpenFiles, linesToSpareForMatches)

        //2) THEN FIND ALL THE CLOSED FILES YOU CAN SHOW IN ORDER OF FILE INDEX
        top := searchManager.matchIndexAtTopOfWindow
        bottom := searchManager.matchIndexAtTopOfWindow + ttyHeight - 1
        for fileIndex := top; fileIndex <= bottom; fileIndex++ {
            if len(searchManager.filesWithMatches) <= fileIndex {
                //have reached end of matched files in the case that
                //they're aren't enough matches files for scroll bar
                break
            } else if linesToSpareForMatches == 0 {
                log.Printf("0 lines left for closed files.")
                break
            }
            file := searchManager.filesWithMatches[fileIndex]
            if !file.isOpen {
                filesInWindowIndeces = append(filesInWindowIndeces, fileIndex)
                linesToSpareForMatches --
            }
        }

        //3) SORT BY FILE INDEX AFTER FINDING BOTH OPEN AND CLOSED FILES
        sort.Ints(filesInWindowIndeces)

        //4) THEN PRINT ALL THE FILES IN WINDOW INDECES
        for _, fileIndex := range filesInWindowIndeces {
            fileWithMatches := searchManager.filesWithMatches[fileIndex]
            if fileIndex == searchManager.selectedMatchIndex {
                fileWithMatches.isSelected = true
            } else {
                fileWithMatches.isSelected = false
            }
            ut.PrintNewLine()
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
    percentMatchesInWindow := float64(ttyHeight) / float64(len(searchManager.filesWithMatches))
    heightOfScrollBar := ut.Round(percentMatchesInWindow * float64(ttyHeight))
    log.Printf("Calculated scroll bar height to be %v lines (%.2f%% of tty height %v).", heightOfScrollBar, percentMatchesInWindow, ttyHeight)
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

func (searchManager *SearchManager) toggleIfMatchIsOpen(fileToToggleIndex int) {
    isNowOpen := !searchManager.filesWithMatches[fileToToggleIndex].isOpen
    searchManager.filesWithMatches[fileToToggleIndex].isOpen = isNowOpen

    if isNowOpen {
        //if file is now open, add file index to queue
        searchManager.openFileIndexQueue = append(searchManager.openFileIndexQueue, fileToToggleIndex)
    } else {
        //if file is now closed remove file index from queue
        for loopIndex, fileIndex := range searchManager.openFileIndexQueue {
            if fileToToggleIndex == fileIndex {
                searchManager.openFileIndexQueue = append(searchManager.openFileIndexQueue[:loopIndex], searchManager.openFileIndexQueue[loopIndex+1:]...)
            }
        }   
    }
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
        matchIndexToToggle := searchManager.selectedMatchIndex
        searchManager.toggleIfMatchIsOpen(matchIndexToToggle)
        searchManager.renderSearchMatches()

    } else {
        //not chars being added to search term or a recognized command
        return
    }
    searchManager.renderSearchTerm()
    searchManager.renderScrollBar()
}

func init() {
    ut.SetUpLogging()
}

func main() {
    log.Printf("STARTING MAIN DEBOUNCE_GREP PROGRAM.\n\n\n")
    searchManager := NewSearchManager()
    searchManager.listenToStdinAndSearchFiles()
}
