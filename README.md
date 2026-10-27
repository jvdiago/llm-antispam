# llm-antispam

## What is this?

A small Go program that connects via IMAP to your mail and uses LLMs to categorize emails as Spam/Ham
It uses KEV like LLMs with the v1/systemone API. It does not work with normal LLMs (langchain support has been removed)

## How it works

Based on the rules, it fetches the non-read messages if they have not been previously processed (the program stores a file per folder with the last UID processed). Strips all headers, images and HTML and categorizes it as SPAM/HAM by an LLM. Then, based on the rules it moves the email to the destination folder if the rule is satisfied.

## Running it

Export this three ENV vars to connect via IMAP

 IMAP_SERVER=
 IMAP_USER=
 IMAP_PASSWORD=

copy the config_sample.yaml, edit it and use the -config flag to run the service

```llm-antispam -config ./config.yaml```

The program outputs all the logs to stdout and stderr. It is meant to be run with systemd

## Test email score
Using the CLI verb `score` we can get the score for a specific email in EML format:

```
llm-antispam score --config ./config_sample.yaml spam.eml
```
