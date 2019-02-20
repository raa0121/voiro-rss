package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/mmcdole/gofeed"
)

type MyMainWindow struct {
	walk.MainWindow
	lb           *walk.ListBox
	le           *walk.LineEdit
	pb           *walk.PushButton
	cb           *walk.ComboBox
	te           *walk.TextEdit
	RssUrl       string
	prevFilePath string

	cfg *config
}

type config struct {
	Vrx Vrx   `toml:"vrx"`
	Rss []Rss `toml:"rss"`
}

type Vrx struct {
	Path string `toml:"path"`
}

type Rss struct {
	Name string `toml:"name"`
	Url  string `toml:"url"`
}

func (cfg *config) load() error {
	cfg.Vrx.Path = "C:\\Program Files"
	cfg.Rss = KnownRss()

	dir := os.Getenv("APPDATA")
	if dir == "" {
		dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data")
	}
	dir = filepath.Join(dir, "VroidRSS")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("cannot create directory: %v", err)
	}
	file := filepath.Join(dir, "config.toml")

	_, err := os.Stat(file)
	if err == nil {
		// ファイルが存在している場合
		_, err := toml.DecodeFile(file, &cfg)
		if err != nil {
			return err
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func (cfg *config) save() error {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		dir = filepath.Join(os.Getenv("USERPROFILE"), "Application Data")
	}
	dir = filepath.Join(dir, "VroidRSS")
	file := filepath.Join(dir, "config.toml")
	f, err := os.OpenFile(file, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func (mw *MyMainWindow) openAction_Triggered() {
	if err := mw.openVRX(); err != nil {
		log.Print(err)
	}
}

func (mw *MyMainWindow) addAction_Triggered() {
	var rss Rss
	cmd, err := dialogRss(mw, &rss)
	if err != nil {
		log.Print(err)
	} else if cmd == walk.DlgCmdOK {
		mw.cfg.Rss = append(mw.cfg.Rss, rss)
		_ = mw.cb.SetModel(mw.cfg.Rss)
		_ = mw.cb.SetCurrentIndex(0)
	}
}

func (mw *MyMainWindow) openVRX() error {
	dlg := new(walk.FileDialog)

	dlg.FilePath = mw.prevFilePath
	dlg.Filter = "vrx.exe (vrx.exe)|vrx.exe"
	dlg.Title = "Select vrx.exe"

	if ok, err := dlg.ShowOpen(mw); err != nil {
		log.Fatal(err)
		return err
	} else if !ok {
		log.Fatal(ok)
		return nil
	}
	mw.prevFilePath = dlg.FilePath
	if err := mw.le.SetText(dlg.FilePath); err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func KnownRss() []Rss {
	return []Rss{
		{"NHK", "https://www3.nhk.or.jp/rss/news/cat0.xml"},
	}
}

func dialogRss(mw walk.Form, rss *Rss) (int, error) {
	var dlg *walk.Dialog
	var db *walk.DataBinder
	var acceptPB, cancelPB *walk.PushButton

	return Dialog{
		AssignTo:      &dlg,
		Title:         "RSSの追加",
		DefaultButton: &acceptPB,
		CancelButton:  &cancelPB,
		DataBinder: DataBinder{
			AssignTo:       &db,
			Name:           "rss",
			DataSource:     rss,
			ErrorPresenter: ToolTipErrorPresenter{},
		},
		MinSize: Size{Width: 300, Height: 100},
		Layout:  VBox{},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 2},
				Children: []Widget{
					Label{
						Text: "Title:",
					},
					LineEdit{
						Text: Bind("Name"),
					},
					Label{
						Text: "URL:",
					},
					LineEdit{
						Text: Bind("Url"),
					},
				},
			},
			Composite{
				Layout: HBox{},
				Children: []Widget{
					HSpacer{},
					PushButton{
						AssignTo: &acceptPB,
						Text:     "OK",
						OnClicked: func() {
							if err := db.Submit(); err != nil {
								log.Print(err)
								return
							}

							dlg.Accept()
						},
					},
					PushButton{
						AssignTo:  &cancelPB,
						Text:      "Cancel",
						OnClicked: func() { dlg.Cancel() },
					},
				},
			},
		},
	}.Run(mw)
}

func (mw *MyMainWindow) log(msg string) error {
	var err error
	mw.Synchronize(func() {
		beforeText := mw.te.Text()
		if beforeText == "" {
			err = mw.te.SetText(msg + "\r\n")
		} else {
			err = mw.te.SetText(beforeText + msg + "\r\n")
		}
	})
	return err
}

func (mw *MyMainWindow) saveAction_Triggered() {
	mw.cfg.Vrx.Path = mw.prevFilePath
	if err := mw.cfg.save(); err != nil {
		log.Fatal(err)
	}
}

func (mw *MyMainWindow) playAction_Triggered() {
	var url string
	fp := gofeed.NewParser()
	for _, i := range mw.cfg.Rss {
		if i.Name == mw.cb.Text() {
			url = i.Url
		}
	}
	feed, err := fp.ParseURL(url)
	if err != nil {
		log.Fatal(err)
	}

	mw.SetEnabled(false)
	go func() {
		defer func() {
			mw.Synchronize(func() {
				mw.SetEnabled(true)
			})
		}()
		fmt.Println(feed.Items)
		for _, item := range feed.Items {
			mw.log(item.Title)
			mw.log("  " + item.Description)
			err := exec.Command(mw.cfg.Vrx.Path, item.Title).Run()
			if err != nil {
				mw.log(fmt.Sprintf("execute fail %+v.\n", err))

			}
			time.Sleep(2 * time.Second)
			err = exec.Command(mw.cfg.Vrx.Path, item.Description).Run()
			if err != nil {
				mw.log(fmt.Sprintf("execute fail %+v.\n", err))
			}
			time.Sleep(5 * time.Second)
		}
	}()
}

func main() {
	var cfg config
	err := cfg.load()
	if err != nil {
		log.Fatal(err)
	}
	mw := new(MyMainWindow)
	mw.cfg = &cfg

	if _, err := (MainWindow{
		Title:   "VoiroRSS",
		MinSize: Size{Width: 500, Height: 75},
		Layout:  VBox{},
		Children: []Widget{
			Composite{
				Layout: Grid{Columns: 3},
				Children: []Widget{
					Label{
						Text: "RSS の URL",
					},
					ComboBox{
						AssignTo:      &mw.cb,
						Value:         Bind(mw.RssUrl, SelRequired{}),
						BindingMember: "Url",
						DisplayMember: "Name",
						Model:         cfg.Rss,
						CurrentIndex:  0,
					},
					PushButton{
						Text:      "追加",
						OnClicked: mw.addAction_Triggered,
					},
				},
			},
			Composite{
				Layout: Grid{Columns: 3},
				Children: []Widget{
					Label{
						Text: "vrx.exe のパス",
					},
					LineEdit{
						AssignTo: &mw.le,
						Text:     cfg.Vrx.Path,
					},
					PushButton{
						Text:      "開く",
						OnClicked: mw.openAction_Triggered,
					},
				},
			},
			PushButton{
				Text:      "保存",
				OnClicked: mw.saveAction_Triggered,
			},
			PushButton{
				Text:      "取得・再生",
				OnClicked: mw.playAction_Triggered,
			},
			TextEdit{
				AssignTo: &mw.te,
			},
		},
	}.Run()); err != nil {
		log.Fatal(err)
	}
}
