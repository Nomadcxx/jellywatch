package radarr

import (
	"fmt"
	"net/url"
)

func (c *Client) GetMovies() ([]Movie, error) {
	var movies []Movie
	if err := c.get("/api/v3/movie", &movies); err != nil {
		return nil, fmt.Errorf("getting movies: %w", err)
	}
	return movies, nil
}

func (c *Client) GetMovie(id int) (*Movie, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/%d", id)
	var movie Movie
	if err := c.get(endpoint, &movie); err != nil {
		return nil, fmt.Errorf("getting movie %d: %w", id, err)
	}
	return &movie, nil
}

func (c *Client) LookupMovie(term string) ([]Movie, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/lookup?term=%s", url.QueryEscape(term))
	var movies []Movie
	if err := c.get(endpoint, &movies); err != nil {
		return nil, fmt.Errorf("looking up movie %q: %w", term, err)
	}
	return movies, nil
}

func (c *Client) LookupMovieByTmdbID(tmdbID int) (*Movie, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/lookup/tmdb?tmdbId=%d", tmdbID)
	var movie Movie
	if err := c.get(endpoint, &movie); err != nil {
		return nil, fmt.Errorf("looking up movie by TMDB ID %d: %w", tmdbID, err)
	}
	return &movie, nil
}

func (c *Client) LookupMovieByImdbID(imdbID string) (*Movie, error) {
	endpoint := fmt.Sprintf("/api/v3/movie/lookup/imdb?imdbId=%s", url.QueryEscape(imdbID))
	var movie Movie
	if err := c.get(endpoint, &movie); err != nil {
		return nil, fmt.Errorf("looking up movie by IMDB ID %s: %w", imdbID, err)
	}
	return &movie, nil
}

func (c *Client) GetMovieByTmdbID(tmdbID int) (*Movie, error) {
	movies, err := c.GetMovies()
	if err != nil {
		return nil, err
	}

	for _, movie := range movies {
		if movie.TmdbID == tmdbID {
			return &movie, nil
		}
	}

	return nil, fmt.Errorf("movie with TMDB ID %d not found in library", tmdbID)
}

func (c *Client) GetMovieByImdbID(imdbID string) (*Movie, error) {
	movies, err := c.GetMovies()
	if err != nil {
		return nil, err
	}

	for _, movie := range movies {
		if movie.ImdbID == imdbID {
			return &movie, nil
		}
	}

	return nil, fmt.Errorf("movie with IMDB ID %s not found in library", imdbID)
}

func (c *Client) DeleteMovie(id int, deleteFiles, addImportExclusion bool) error {
	endpoint := fmt.Sprintf("/api/v3/movie/%d?deleteFiles=%t&addImportExclusion=%t",
		id, deleteFiles, addImportExclusion)
	if err := c.delete(endpoint); err != nil {
		return fmt.Errorf("deleting movie %d: %w", id, err)
	}
	return nil
}

func (c *Client) GetMovieFiles(movieID int) ([]MovieFile, error) {
	endpoint := fmt.Sprintf("/api/v3/moviefile?movieId=%d", movieID)
	var files []MovieFile
	if err := c.get(endpoint, &files); err != nil {
		return nil, fmt.Errorf("getting movie files for movie %d: %w", movieID, err)
	}
	return files, nil
}

func (c *Client) DeleteMovieFile(id int) error {
	endpoint := fmt.Sprintf("/api/v3/moviefile/%d", id)
	if err := c.delete(endpoint); err != nil {
		return fmt.Errorf("deleting movie file %d: %w", id, err)
	}
	return nil
}
