/*
 * Copyright (c) 2022 NetLOX Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package loxilib

import (
    "log"
    "os"
    "fmt"
)

type LogLevelT int

const (
    LOG_EMERG LogLevelT = iota
    LOG_ALERT
    LOG_CRITICAL
    LOG_ERROR
    LOG_WARNING
    LOG_NOTICE
    LOG_INFO
    LOG_DEBUG
)

var (
    LogTTY       bool
    CurrLogLevel LogLevelT
    LogItEmer	 *log.Logger
    LogItAlert   *log.Logger
    LogItCrit    *log.Logger
    LogItErr     *log.Logger
    LogItWarn    *log.Logger
    LogItNotice  *log.Logger
    LogItInfo    *log.Logger
    LogItDebug   *log.Logger   
 ) 

func LogItInit(logFile string, logLevel LogLevelT, toTTY bool) {
    file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
    if err != nil {
        log.Fatal(err)
    }

    if logLevel < LOG_EMERG || logLevel > LOG_DEBUG {
        log.Fatal(err)
    }

    CurrLogLevel = logLevel
    LogTTY = toTTY
    LogItEmer  = log.New(file, "EMER: ", log.Ldate|log.Ltime)
    LogItAlert = log.New(file, "ALRT: ", log.Ldate|log.Ltime)
    LogItCrit  = log.New(file, "CRIT: ", log.Ldate|log.Ltime)
    LogItErr   = log.New(file, "ERR:  ", log.Ldate|log.Ltime)
    LogItWarn  = log.New(file, "WARN: ", log.Ldate|log.Ltime)
    LogItNotice= log.New(file, "NOTI: ", log.Ldate|log.Ltime)
    LogItInfo  = log.New(file, "INFO: ", log.Ldate|log.Ltime)
    LogItDebug = log.New(file, "DBG:  ", log.Ldate|log.Ltime)
}

// LogIt uses Printf format internally
// Arguments are considered in-line with fmt.Printf.
func LogIt(l LogLevelT, format string, v ...interface{}) {
    if l < 0 || l > CurrLogLevel {
        return
    }
    switch (l) {
    case LOG_EMERG:
        LogItEmer.Printf(format, v...)
        break
    case LOG_ALERT:
        LogItAlert.Printf(format, v...)
        break
    case LOG_CRITICAL:
        LogItCrit.Printf(format, v...)
        break
    case LOG_ERROR:
        LogItErr.Printf(format, v...)
        break
    case LOG_WARNING:
        LogItWarn.Printf(format, v...)
        break
    case LOG_NOTICE:
        LogItNotice.Printf(format, v...)
        break
    case LOG_INFO:
        LogItInfo.Printf(format, v...)
        break
    case LOG_DEBUG:
        LogItDebug.Printf(format, v...)
        break;
    default:
        break;
    }

    if LogTTY == true {
        fmt.Printf(format, v...)
    }
}