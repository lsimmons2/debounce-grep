package config

import (
    "log"
    ut "debounce_grep/utilities"
    "flag"
    "os"
    "strconv"
    "strings"
)

//Each config option is represented by one of three types of structs:
//IntConfigOption, StringConfigOption, or BooleanConfigOption. Each looks
//for flags first, then environmental variables, and if neither are found
//returns a default value. Each has similar but different enough behavior
//that I don't think inheritance is necessarily merited.

var (
    //define variables for flags that take multiple options
    dirsToSearchFlag MultiValueFlag
    fileShebangsFlag MultiValueFlag
    toIgnoreFlag MultiValueFlag

    intOptions = []IntConfigOption {
        IntConfigOption {
            name: "debounceTimeMs",
            defaultValue: 200,
            envVariableName: "DEBOUNCE_GREP_DEBOUNCE_TIME_MS",
            flagSymbol: "ms",
            description: "Time between debounced searches, in MS.",
        },
        IntConfigOption {
            name: "maxLinesToPrintPerFile",
            defaultValue: 5,
            envVariableName: "DEBOUNCE_GREP_MAX_LINES_PER_FILE",
            flagSymbol: "lines",
            description: "Max number of lines of matches to print per file.",
        },
    }

    stringOptions = []StringConfigOption {
        StringConfigOption {
            name: "dirsToSearch",
            defaultValue: []string{ut.GetCurrentWorkingDir()},
            envVariableName: "DEBOUNCE_GREP_DIRS_TO_SEARCH",
            flagSymbol: "dir",
            flag: dirsToSearchFlag,
            description: "Directories to search.",
        },
        StringConfigOption {
            name: "fileShebangs",
            defaultValue: []string{},
            envVariableName: "DEBOUNCE_GREP_FILE_SHEBANGS",
            flagSymbol: "shebang",
            flag: fileShebangsFlag,
            description: "Shebangs of files to search.",
        },
        StringConfigOption {
            name: "patternsToIgnore",
            defaultValue: []string{".git", "venv", "node_modules", "bower_components", "*.png", "*.jpg", "*.jpeg", "*.pyc"},
            envVariableName: "DEBOUNCE_GREP_PATTERNS_TO_IGNORE",
            flagSymbol: "ignore",
            flag: toIgnoreFlag,
            description: "Glob patterns of files and directories to ignore.",
        },
    }

    booleanOptions = []BooleanConfigOption {
        BooleanConfigOption {
            name: "shouldPrintWholeLines",
            defaultValue: false,
            envVariableName: "DEBOUNCE_GREP_SHOULD_PRINT_WHOLE_LINES",
            flagSymbol: "whole-lines",
            description: "If should print whole lines of matches as opposed to truncating them at end of tty.",
        },
    }

    Options = ConfigOptions{
        intOptions: intOptions,
        stringOptions: stringOptions,
        booleanOptions: booleanOptions,
    }

    //map that will ultimately be used to access values of config options
    Values = make(map[string]interface{})
)



type ConfigOptions struct {
    intOptions []IntConfigOption
    stringOptions []StringConfigOption
    booleanOptions []BooleanConfigOption
}

func (configOptions *ConfigOptions) parseAndSaveValues() {
    //need to define all flag parsers before calling flag.Parse()
    configOptions.defineFlags()
    flag.Parse()
    //loop these individually since they're slices of different types
    for _, intOption := range configOptions.intOptions {
        Values[intOption.name] = intOption.getValue()
    }
    for _, stringOption := range configOptions.stringOptions {
        Values[stringOption.name] = stringOption.getValue()
    }
    for _, booleanOption := range configOptions.booleanOptions {
        Values[booleanOption.name] = booleanOption.getValue()
    }
}

func (configOptions *ConfigOptions) defineFlags() {
    var intOption *IntConfigOption
    for i, _ := range configOptions.intOptions {
        intOption = &configOptions.intOptions[i]
        intOption.flagPointer = flag.Int(intOption.flagSymbol, intOption.defaultValue, intOption.description)
    }
    var stringOption *StringConfigOption
    for i, _ := range configOptions.stringOptions {
        stringOption = &configOptions.stringOptions[i]
        flag.Var(&stringOption.flag, stringOption.flagSymbol, stringOption.description)
    }
    var booleanOption *BooleanConfigOption
    for i, _ := range configOptions.booleanOptions {
        booleanOption = &configOptions.booleanOptions[i]
        booleanOption.flagPointer = flag.Bool(booleanOption.flagSymbol, booleanOption.defaultValue, booleanOption.description)
    }
}

type IntConfigOption struct {
    name string
    defaultValue int
    envVariableName string
    flagSymbol string
    description string
    flagPointer *int
}

func (option *IntConfigOption) getValue() int {
    //1) check flag
    flagValue := *option.flagPointer
    if flagValue > 0 {
        log.Printf("Value %v retrieved for config option %v from flag %v.", flagValue, option.name, option.flagSymbol)
        return flagValue
    }
    //2) check environmental variable
    envVarValueString := os.Getenv(option.envVariableName)
    envVarValueInt, err := strconv.Atoi(envVarValueString)
    if err != nil {
        //3) return default if neither flag or environmental variable provided
        log.Printf("Either no value provided or value provided for config option %v from environmental variable %v could not be converted into int, returning default value %v.", option.name, option.envVariableName, option.defaultValue)
        return option.defaultValue
    }
    log.Printf("Value %v retrieved for config option %v from environmental variable %v.", envVarValueInt, option.name, option.envVariableName)
    return envVarValueInt
}

type StringConfigOption struct {
    name string
    defaultValue []string
    envVariableName string
    flagSymbol string
    flag MultiValueFlag
    description string
    flagPointer *string
}

func (option *StringConfigOption) getValue() []string {
    //1) check flag
    flagValue := option.flag
    if len(flagValue) > 0 {
        log.Printf("Value %v retrieved for config option %v from flag %v.", flagValue, option.name, option.flagSymbol)
        return flagValue
    }
    //2) check environmental variable
    envValue := os.Getenv(option.envVariableName)
    if len (envValue) == 0 {
        //3) return default if neither flag or environmental variable provided
        log.Printf("No environmental variable for config option %v detected, returning default value of %v.", option.envVariableName, option.defaultValue)
        return option.defaultValue
    }
    envVariableList := strings.Split(envValue, ":")
    log.Printf("Returning environmental variable value %v for %v config option.", envVariableList, option.envVariableName)
    return envVariableList
}

//for defining flags with multiple possible values, you need to create
//a custom type to implement with flag.Var()
type MultiValueFlag []string

func (multiValueFlag *MultiValueFlag) String() string {
    return "my string representation"
}

func (multiValueFlag *MultiValueFlag) Set(value string) error {
    *multiValueFlag = append(*multiValueFlag, value)
    return nil
}

type BooleanConfigOption struct {
    name string
    defaultValue bool
    envVariableName string
    flagSymbol string
    description string
    flagPointer *bool
}

func (option *BooleanConfigOption) getValue() bool {
    //because it doesn't seem that the default golang flag library
    //can distinguish between default flags and no flags passed at
    //all, all boolean flags will have to be specified as truthy:
    //you won't be able to pass a boolean flag as false
    flagValue := *option.flagPointer
    if flagValue == false {
        log.Printf("No truthy boolean retrieved for config option %v from flag %v.", option.name, option.flagSymbol)
    } else {
        log.Printf("Truthy value retrieved for config option %v from flag %v.", option.name, option.flagSymbol)
        return true
    }
    envValueString := os.Getenv(option.envVariableName)
    envValue, _ := strconv.ParseBool(envValueString)
    log.Printf("Value %v retrieved for config option %v from environmental variable %v.", envValue, option.name, option.envVariableName)
    return envValue
}

func init(){
    ut.SetUpLogging()
    Options.parseAndSaveValues()
}
