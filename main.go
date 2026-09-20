package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

// Regex pour vérifier si le nom (sans extension) contient UNIQUEMENT des chiffres
var numericFileNameRegex = regexp.MustCompile(`^\d+$`)

func main() {
	targetURL := flag.String("link", "", "sushiscan link to a manga chapter") // Remplacez par votre URL
	outputDir := flag.String("directory", "", "download directory")

	flag.Parse()

	if err := os.MkdirAll(*outputDir, os.ModePerm); err != nil {
		fmt.Printf("Erreur création dossier: %v\n", err)
		return
	}

	// 1. Récupérer la page HTML
	req, err := http.NewRequest("GET", *targetURL, nil)
	if err != nil {
		fmt.Printf("Erreur requête: %v\n", err)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Erreur lors de la visite: %v\n", err)
		return
	}
	defer resp.Body.Close()

	baseURL, err := url.Parse(*targetURL)
	if err != nil {
		fmt.Printf("URL invalide: %v\n", err)
		return
	}

	// 2. Analyser le HTML et extraire les URLs des <img>
	doc, err := html.Parse(resp.Body)
	if err != nil {
		fmt.Printf("Erreur analyse HTML: %v\n", err)
		return
	}

	var imageUrls []string
	var extractImgSrc func(*html.Node)
	extractImgSrc = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			var src string
			for _, attr := range n.Attr {
				if attr.Key == "src" && attr.Val != "" {
					src = attr.Val
					break
				}
				if attr.Key == "data-src" && src == "" {
					src = attr.Val
				}
			}

			if src != "" {
				imgURL, err := url.Parse(src)
				// fmt.Println(imgURL)
				if err == nil {
					absURL := baseURL.ResolveReference(imgURL).String()

					// Extraire le nom de fichier sans paramètres d'URL (?v=...)
					imageUrls = append(imageUrls, absURL)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extractImgSrc(c)
		}
	}
	extractImgSrc(doc)

	fmt.Printf("Trouvé %d image(s) numérique(s) correspondante(s). Début du téléchargement...\n", len(imageUrls))

	// 3. Télécharger les images en parallèle
	var wg sync.WaitGroup
	for i, imgURL := range imageUrls {
		wg.Add(1)
		go func(index int, img string) {
			defer wg.Done()

			filename := filepath.Base(img)
			if idx := strings.Index(filename, "?"); idx != -1 {
				filename = filename[:idx]
			}

			destPath := filepath.Join(*outputDir, filename)

			if err := downloadFile(client, img, destPath); err != nil {
				fmt.Printf("[%d/%d] Erreur (%s) : %v\n", index+1, len(imageUrls), img, err)
			} else {
				fmt.Printf("[%d/%d] Téléchargé : %s\n", index+1, len(imageUrls), filename)
			}
		}(i, imgURL)
	}

	wg.Wait()
	fmt.Println("Téléchargement terminé !")
}

func downloadFile(client *http.Client, imgURL, destPath string) error {
	req, err := http.NewRequest("GET", imgURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("statut HTTP %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
