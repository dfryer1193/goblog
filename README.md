# goblog

A blogging platform backend designed to consume Markdown articles and produce
blog posts.

Supports comments.

Does not support user login (yet)

## Post Repo Setup

Currently, the project expects a specific post repo structure:

```text
<blog_posts>/
    |-posts/
    |   |-001_first_post.md
    |   |-002_second_post.md
    |   |-003_third_post.md
    |   |-...
    |   `-nnn_Nth_post.md
    `-images/ 
        |-some_image.jpg
        `-another_image.png
```

The webhook handler will generate html files from the markdown files, prefixing
any links with the preconfigured blog domain. (currently defaults to my
personal blog at `https://blog.werewolves.fyi`)

Images will exist at `http://<domain>/images/`. The document AST will handle
pointing relative links at the right place, including handling `./` and `../`
without allowing http server path traversal. Given the strict project
structure, the parsing logic assumes that every link node points to a post, and
every image node points to an image.

### Examples

This markdown

```markdown
This text contains a [relative link](./README.md).
It also contains ![an image](../images/test_image.png)
```

...generates this html (css class information is omitted for brevity):

```html
This text contains a <a href="https://blog.werewolves.fyi/README">relative link</a>.
It also contains <img src="https://blog.werewolves.fyi/test_image.png" alt="an image"/>
```
