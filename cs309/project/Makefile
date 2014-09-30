.PHONY: static

static: js

js:
	find public/js -type f -path "*.coffee" -exec coffee -c {} \;

clean:
	find public/js -type f -path "*.js" -exec rm {} \;
