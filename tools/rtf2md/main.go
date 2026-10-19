// rtf2md liest JAPA-Pressemeldungen (RTF) und ersetzt in der passenden Ausschreibungsseite
// (content/pruefungen/<typ><nr>.md) den Text durch die Ergebnisse. Das Front Matter bleibt erhalten.
//
// Aufruf: go run . [-dir ../../content/pruefungen] [Dateien.rtf ...]
// Ohne Dateiangabe werden Downloads\Meldung_Press*.rtf verwendet.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type entry struct {
	Name, Rasse, Nr, Gewoelft, Fuehrer, Ort, Punkte, Preis, Laut string
}

type result struct {
	Typ, Datum, DatumBis, Ort, Zahlen, Wetter string
	Date                                      time.Time
	Entries                                   []entry
	HasPreis                                  bool
}

var intro = map[string]string{
	"HZP":  "HZP ohne Spurarbeit",
	"VGP":  "Verbandsgebrauchsprüfung mit Übernachtfährte",
	"VPS":  "Verbandsprüfung nach dem Schuß - mit und ohne Fuchs",
	"VStP": "Verbandsstöberprüfung - vom Führer begleitet oder vom Stand geschnallt",
	"VJP":  "Verbandsjugendprüfung",
}

var (
	reStart    = regexp.MustCompile(`^[A-Za-z]+ am \d`)
	reHeader   = regexp.MustCompile(`^([A-Za-z]+) am (\d{2}\.\d{2}\.\d{4})(?: bis (\d{2}\.\d{2}\.\d{4}))? um (.+?),?\s+(gem\..*Hunde)\s*$`)
	reDog      = regexp.MustCompile(`^(.+), ([^,]+), ([^,]+), gew\. (.+)$`)
	reOwner    = regexp.MustCompile(`^F: (.+?), (\d{5} .+?), (-|\d+) P\.(?:, (.*))?$`)
	reSpace    = regexp.MustCompile(`\s+`)
	reTermin   = regexp.MustCompile(`(?m)^termin:\s*"(\d{4}-\d{2}-\d{2})"`)
	reOnline   = regexp.MustCompile(`(?m)^onlineMeldung:`)
	reFrontEnd = regexp.MustCompile(`(?s)\A(---\r?\n.*?\r?\n---)`)
)

var weekdays = []string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}
var months = []string{"", "Januar", "Februar", "März", "April", "Mai", "Juni", "Juli", "August", "September", "Oktober", "November", "Dezember"}

func parse(path string) (*result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := strings.NewReplacer(cellSep, "\n", rowSep, "\n").Replace(rtfToText(data))

	var lines []string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(reSpace.ReplaceAllString(strings.ReplaceAll(l, "\u00a0", " "), " "))
		if l != "" {
			lines = append(lines, l)
		}
	}

	r := &result{}
	i := 0
	for ; i < len(lines); i++ {
		if !reStart.MatchString(lines[i]) {
			continue
		}
		h := lines[i]
		// Die Kopfzeile kann in der Datei umbrochen sein.
		for !strings.HasSuffix(h, "Hunde") && i+1 < len(lines) {
			i++
			h += " " + lines[i]
		}
		m := reHeader.FindStringSubmatch(h)
		if m == nil {
			return nil, fmt.Errorf("%s: Kopfzeile nicht lesbar: %q", path, h)
		}
		r.Typ, r.Datum, r.DatumBis, r.Ort, r.Zahlen = m[1], m[2], m[3], m[4], m[5]
		i++
		break
	}
	if r.Typ == "" {
		return nil, fmt.Errorf("%s: keine Kopfzeile gefunden", path)
	}
	d, err := time.Parse("02.01.2006", r.Datum)
	if err != nil {
		return nil, err
	}
	r.Date = d

	for ; i < len(lines); i++ {
		l := lines[i]
		if strings.HasPrefix(l, "JAPA ") {
			break
		}
		if strings.HasPrefix(l, "Wetter:") {
			r.Wetter = l
			continue
		}
		dm := reDog.FindStringSubmatch(l)
		if dm == nil || i+1 >= len(lines) {
			continue
		}
		om := reOwner.FindStringSubmatch(lines[i+1])
		if om == nil {
			return nil, fmt.Errorf("%s: Führerzeile nicht lesbar: %q", path, lines[i+1])
		}
		i++
		e := entry{Name: dm[1], Rasse: dm[2], Nr: dm[3], Gewoelft: dm[4], Fuehrer: om[1], Ort: om[2], Punkte: om[3]}
		for _, p := range strings.Split(om[4], ", ") {
			p = strings.TrimSpace(p)
			switch {
			case p == "":
			case strings.HasSuffix(p, " Preis"):
				e.Preis = strings.TrimSuffix(p, " Preis")
				r.HasPreis = true
			default:
				e.Laut = p
			}
		}
		r.Entries = append(r.Entries, e)
	}
	if len(r.Entries) == 0 {
		return nil, fmt.Errorf("%s: keine Ergebnisse gefunden", path)
	}
	return r, nil
}

func cell(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "|", "/")
}

func renderBody(r *result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s**\n\n", intro[r.Typ])
	fmt.Fprintf(&b, "Am %s den %d. %s\n\n", weekdays[r.Date.Weekday()], r.Date.Day(), months[r.Date.Month()])
	if r.DatumBis != "" {
		fmt.Fprintf(&b, "%s am %s bis %s um %s\n\n%s\n\n", r.Typ, r.Datum, r.DatumBis, r.Ort, r.Zahlen)
	} else {
		fmt.Fprintf(&b, "%s am %s um %s, %s\n\n", r.Typ, r.Datum, r.Ort, r.Zahlen)
	}
	if r.Wetter != "" {
		b.WriteString(r.Wetter + "\n\n")
	}
	b.WriteString("Ergebnisse:  \n{{<table \"table table-dark table-striped table-bordered\" >}}\n\n")
	if r.HasPreis {
		b.WriteString("  | Name | Rasse | # | gewölft | Führer | Ort | Punkte | Preis | Laut |\n  | ---- | ----- |-|-------|------|---|------|----|----|\n")
	} else {
		b.WriteString("  | Name | Rasse | # | gewölft | Führer | Ort | Punkte | Laut |\n  | ---- | ----- |-|-------|------|---|------|----|\n")
	}
	for _, e := range r.Entries {
		preis, laut := e.Preis, e.Laut
		if r.HasPreis && preis == "" {
			if laut == "nicht bestanden" {
				preis, laut = laut, "-"
			} else {
				preis = "-"
			}
		}
		cols := []string{e.Name, e.Rasse, e.Nr, e.Gewoelft, e.Fuehrer, e.Ort, e.Punkte}
		if r.HasPreis {
			cols = append(cols, preis)
		}
		cols = append(cols, laut)
		for i := range cols {
			cols[i] = cell(cols[i])
		}
		b.WriteString("|" + strings.Join(cols, "| ") + "|\n")
	}
	b.WriteString("\n{{</table>}}\n")
	return b.String()
}

// findPage sucht die Ausschreibung gleichen Typs mit dem nächstgelegenen Termin (max. 7 Tage Abstand).
func findPage(dir string, r *result) (string, error) {
	re := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(r.Typ) + `\d*\.md$`)
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	best, bestDiff := "", 8.0
	for _, f := range files {
		if f.IsDir() || !re.MatchString(f.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return "", err
		}
		m := reTermin.FindSubmatch(data)
		if m == nil {
			continue
		}
		t, err := time.Parse("2006-01-02", string(m[1]))
		if err != nil {
			continue
		}
		diff := r.Date.Sub(t).Hours() / 24
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best, bestDiff = filepath.Join(dir, f.Name()), diff
		}
	}
	if best == "" {
		return "", fmt.Errorf("keine Ausschreibung für %s am %s in %s gefunden", r.Typ, r.Datum, dir)
	}
	return best, nil
}

func update(path string, r *result) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m := reFrontEnd.FindString(string(data))
	if m == "" {
		return fmt.Errorf("%s: kein Front Matter", path)
	}
	fm := reTermin.ReplaceAllString(m, `termin: "`+r.Date.Format("2006-01-02")+`"`)
	fm = reOnline.ReplaceAllString(fm, "#onlineMeldung:")
	out := strings.ReplaceAll(fm+"\n\n"+renderBody(r), "\r\n", "\n")
	// Gemischte Zeilenenden bringen Hugo beim Rendern durcheinander.
	if bytes.Contains(data, []byte("\r\n")) {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func main() {
	dir := flag.String("dir", filepath.Join("..", "..", "content", "pruefungen"), "Verzeichnis mit den Ausschreibungen")
	flag.Parse()

	files := flag.Args()
	if len(files) == 0 {
		home, _ := os.UserHomeDir()
		files, _ = filepath.Glob(filepath.Join(home, "Downloads", "Meldung_Press*.rtf"))
	}

	for _, f := range files {
		r, err := parse(f)
		if err == nil {
			var page string
			if page, err = findPage(*dir, r); err == nil {
				if err = update(page, r); err == nil {
					fmt.Printf("%s -> %s\n", filepath.Base(f), page)
				}
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Fehler:", err)
		}
	}
}
