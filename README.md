# llm-antispam

## What is this?

A small Go program that connects via IMAP to your mail and uses LLMs to categorize emails as Spam/Ham

## How it works

Based on the rules, it fetches the non-read messages if they have not been previously processed (the program stores a file per folder with the last UID processed). Strips all headers, images and HTML and categorizes it as SPAM/HAM by an LLM. Then, based on the rules it moves the email to the destination folder if the rule is satisfied

## Why GO?
This would have been much easier in Python, as Lagnchain has official bindings and better IMAP libraries and the program is not CPU bound, but I just wanted to practice my Go.

## Running it

Export this three ENV vars to connect via IMAP

 IMAP_SERVER=
 IMAP_USER=
 IMAP_PASSWORD=

### Bedrock
When using bedrock provider, export AWS keys
 AWS_ACCESS_KEY_ID=
 AWS_SECRET_ACCESS_KEY=
### OpenAI
When using bedrock provider, export OpenAI keys
 OPENAI_API_KEY=
 
copy the config_sample.yaml, edit it and use the -config flag to run the service

```llm-antispam -config ./config.yaml```

The program outputs all the logs to stdout and stderr. It is meant to be run with systemd

