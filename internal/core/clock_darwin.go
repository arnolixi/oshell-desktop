//go:build darwin

package core

import (
	"github.com/ebitengine/purego/objc"
	"runtime"
	"sync"
	"time"
)

var localClock struct {
	sync.Mutex
	formatter objc.ID
}

func foundationString(text string) objc.ID {
	bytes := append([]byte(text), 0)
	value := objc.ID(objc.GetClass("NSString")).Send(objc.RegisterName("stringWithUTF8String:"), &bytes[0])
	runtime.KeepAlive(bytes)
	return value
}
func foundationText(value objc.ID) string {
	size := objc.Send[uint64](value, objc.RegisterName("lengthOfBytesUsingEncoding:"), uint64(4))
	if size > 8192 {
		return ""
	}
	bytes := make([]byte, size+1)
	if !objc.Send[bool](value, objc.RegisterName("getCString:maxLength:encoding:"), &bytes[0], size+1, uint64(4)) {
		return ""
	}
	return string(bytes[:size])
}
func platformClockText(now time.Time) string {
	if foundationReady() != nil {
		return now.Local().Format("2006-01-02 15:04:05")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	localClock.Lock()
	defer localClock.Unlock()
	if localClock.formatter == 0 {
		formatter := objc.ID(objc.GetClass("NSDateFormatter")).Send(objc.RegisterName("new"))
		locale := objc.ID(objc.GetClass("NSLocale")).Send(objc.RegisterName("localeWithLocaleIdentifier:"), foundationString("en_US_POSIX"))
		calendar := objc.ID(objc.GetClass("NSCalendar")).Send(objc.RegisterName("alloc")).Send(objc.RegisterName("initWithCalendarIdentifier:"), foundationString("gregorian"))
		formatter.Send(objc.RegisterName("setLocale:"), locale)
		formatter.Send(objc.RegisterName("setCalendar:"), calendar)
		calendar.Send(objc.RegisterName("release"))
		formatter.Send(objc.RegisterName("setDateFormat:"), foundationString("yyyy-MM-dd HH:mm:ss"))
		localClock.formatter = formatter
	}
	zone := objc.ID(objc.GetClass("NSTimeZone")).Send(objc.RegisterName("localTimeZone"))
	localClock.formatter.Send(objc.RegisterName("setTimeZone:"), zone)
	date := objc.ID(objc.GetClass("NSDate")).Send(objc.RegisterName("dateWithTimeIntervalSince1970:"), float64(now.UnixNano())/1e9)
	text := localClock.formatter.Send(objc.RegisterName("stringFromDate:"), date)
	return foundationText(text)
}
