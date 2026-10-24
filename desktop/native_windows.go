package main

import (
	"fmt"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var shell32 = syscall.NewLazyDLL("shell32.dll")
var user32 = syscall.NewLazyDLL("user32.dll")

func revealFile(path string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	if err != nil {
		if oe, ok := err.(*ole.OleError); !ok || oe.Code() != 1 {
			return err
		}
	}
	defer ole.CoUninitialize()
	name, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return e
	}
	var pidl uintptr
	hr, _, _ := shell32.NewProc("SHParseDisplayName").Call(uintptr(unsafe.Pointer(name)), 0, uintptr(unsafe.Pointer(&pidl)), 0, 0)
	if int32(hr) < 0 {
		return fmt.Errorf("无法定位启动文件 (0x%x)", hr)
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(pidl)
	hr, _, _ = shell32.NewProc("SHOpenFolderAndSelectItems").Call(pidl, 0, 0, 0)
	if int32(hr) < 0 {
		return fmt.Errorf("打开资源管理器失败 (0x%x)", hr)
	}
	obj, e := oleutil.CreateObject("Shell.Application")
	if e != nil {
		return nil
	}
	defer obj.Release()
	sh, e := obj.QueryInterface(ole.IID_IDispatch)
	if e != nil {
		return nil
	}
	defer sh.Release()
	for attempt := 0; attempt < 8; attempt++ {
		if activateFolder(sh, filepath.Dir(path)) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
func activateFolder(sh *ole.IDispatch, path string) bool {
	wins, e := oleutil.CallMethod(sh, "Windows")
	if e != nil {
		return false
	}
	defer wins.Clear()
	ws := wins.ToIDispatch()
	if ws == nil {
		return false
	}
	count, e := oleutil.GetProperty(ws, "Count")
	if e != nil {
		return false
	}
	defer count.Clear()
	for i := 0; i < int(count.Val); i++ {
		v, e := oleutil.CallMethod(ws, "Item", i)
		if e != nil {
			continue
		}
		ok := activateWindow(v.ToIDispatch(), path)
		v.Clear()
		if ok {
			return true
		}
	}
	return false
}
func activateWindow(win *ole.IDispatch, path string) bool {
	if win == nil {
		return false
	}
	doc, e := oleutil.GetProperty(win, "Document")
	if e != nil {
		return false
	}
	defer doc.Clear()
	document := doc.ToIDispatch()
	if document == nil {
		return false
	}
	folder, e := oleutil.GetProperty(document, "Folder")
	if e != nil {
		return false
	}
	defer folder.Clear()
	folderDispatch := folder.ToIDispatch()
	if folderDispatch == nil {
		return false
	}
	self, e := oleutil.GetProperty(folderDispatch, "Self")
	if e != nil {
		return false
	}
	defer self.Clear()
	selfDispatch := self.ToIDispatch()
	if selfDispatch == nil {
		return false
	}
	p, e := oleutil.GetProperty(selfDispatch, "Path")
	if e != nil {
		return false
	}
	defer p.Clear()
	if !strings.EqualFold(filepath.Clean(p.ToString()), filepath.Clean(path)) {
		return false
	}
	hwnd, e := oleutil.GetProperty(win, "HWND")
	if e != nil {
		return false
	}
	defer hwnd.Clear()
	user32.NewProc("ShowWindow").Call(uintptr(hwnd.Val), 9)
	user32.NewProc("SetForegroundWindow").Call(uintptr(hwnd.Val))
	return true
}
