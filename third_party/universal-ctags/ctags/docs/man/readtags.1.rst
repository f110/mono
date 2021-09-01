.. _readtags(1):

==============================================================
readtags
==============================================================

Find tag file entries matching specified names

:Version: 5.9.0
:Manual group: Universal Ctags
:Manual section: 1

SYNOPSIS
--------
|	**readtags** -h | --help
|	**readtags** (-H | --help-expression) (filter|sorter)
|	**readtags** [OPTION]... ACTION

DESCRIPTION
-----------
The **readtags** program filters, sorts and prints tag entries in a tags file.
The basic filtering is done using **actions**, by which you can list all
regular tags, pseudo tags or regular tags matching specific name. Then, further
filtering and sorting can be done using **post processors**, namely **filter
expressions** and **sorter expressions**.

ACTIONS
-------
``-l``, ``--list``
	List regular tags.

``[-] NAME``
	List regular tags matching NAME.
	"-" as NAME indicates arguments after this as NAME even if they start with -.

``-D``, ``--list-pseudo-tags``
	Equivalent to ``--list-pseudo-tags``.

OPTIONS
-------

Controlling the Tags Reading Behavior
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
The behavior of reading tags can be controlled using these options:

``-t TAGFILE``, ``--tag-file TAGFILE``
	Use specified tag file (default: "tags").

``-s[0|1|2]``, ``--override-sort-detection METHOD``
	Override sort detection of tag file.
	METHOD: unsorted|sorted|foldcase

The NAME action will perform binary search on sorted (including "foldcase")
tags files, which is much faster then on unsorted tags files.

Controlling the NAME Action Behavior
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
The behavior of the NAME action can be controlled using these options:

``-i``, ``--icase-match``
	Perform case-insensitive matching in the NAME action.

``-p``, ``--prefix-match``
	Perform prefix matching in the NAME action.

Controlling the Output
~~~~~~~~~~~~~~~~~~~~~~
By default, the output of readtags contains only the name, input and pattern
field. The Output can be tweaked using these options:

``-d``, ``--debug``
	Turn on debugging output.

``-E``, ``--escape-output``
	Escape characters like tabs in output as described in :ref:`tags(5) <tags(5)>`.

``-e``, ``--extension-fields``
	Include extension fields in output.

``-n``, ``--line-number``
	Also include the line number field when ``-e`` option is give.

About the ``-E`` option: certain characters are escaped in a tags file, to make
it machine-readable. e.g., ensuring no tabs character appear in fields other
than the pattern field. By default, readtags translates them to make it
human-readable, but when utilizing readtags output in a script or a client
tool, ``-E`` option should be used. See :ref:`ctags-client-tools(7) <ctags-client-tools(7)>` for more
discussion on this.

Filtering and Sorting
~~~~~~~~~~~~~~~~~~~~~
Further filtering and sorting on the tags listed by actions are performed using:

``-Q EXP``, ``--filter EXP``
	Filter the tags listed by ACTION with EXP before printing.

``-S EXP``, ``--sorter EXP``
	Sort the tags listed by ACTION with EXP before printing.

These are discussed in the `EXPRESSION`_ section.

Examples
~~~~~~~~
* List all tags in "/path/to/tags":

  .. code-block:: console

     $ readtags -t /path/to/tags -l

* List all tags in "tags" that start with "mymethod":

  .. code-block:: console

     $ readtags -p - mymethod

* List all tags matching "mymethod", case insensitively:

  .. code-block:: console

     $ readtags -i - mymethod

* List all tags start with "myvar", and printing all fields (i.e., the whole line):

  .. code-block:: console

     $ readtags -p -ne - myvar

EXPRESSION
----------
Scheme-style expressions are used for the ``-Q`` and ``-S`` options. For those
who doesn't know Scheme or Lisp, just remember:

* A function call is wrapped in a pair of parenthesis. The first item in it is
  the function/operator name, the others are arguments.
* Function calls can be nested.
* Missing values and boolean false are represented by ``#f``. ``#t`` and all
  other values are considered to be true.

So, ``(+ 1 (+ 2 3))`` means add 2 and 3 first, then add the result with 1.
``(and "string" 1 #t)`` means logical AND on ``"string"``, ``1`` and ``#t``,
and the result is true since there is no ``#f``.

Filtering
~~~~~~~~~
The tag entries that make the filter expression produces true value are printed
by readtags.

The basic operators for filtering are ``eq?``, ``prefix?``, ``suffix?``,
``substr?``, and ``#/PATTERN/``. Language common fields can be accessed using
variables starting with ``$``, e.g., ``$language`` represents the language field.
For example:

* List all tags start with "myfunc" in Python code files:

  .. code-block:: console

     $ readtags -p -Q '(eq? $language "Python")' - myfunc

``downcase`` or ``upcase`` operators can be used to perform case-insensitive
matching:

* List all tags containing "my", case insensitively:

    .. code-block:: console

     $ readtags -Q '(substr? (downcase $name) "my")' -l

We have logical operators like ``and``, ``or`` and ``not``. The value of a
missing field is #f, so we could deal with missing fields:

* List all tags containing "impl" in Python code files, but allow the
  ``language:`` field to be missing:

  .. code-block:: console

     $ readtags -Q '(and (substr? $name "impl")\
                         (or (not $language)\
                             (eq? $language "Python")))' -l

``#/PATTERN/`` is for the case when string predicates (``prefix?``, ``suffix?``,
and ``substr?``) are not enough. You can use "Posix extended regular expression"
as PATTERN.

* List all tags inherits from the class "A":

  .. code-block:: console

     $ readtags -Q '(#/(^|,) ?A(,|$)/ $inherits)' -l

Here ``$inherits`` is a comma-separated class list like "A,B,C", "P, A, Q", or
just "A". Notice that this filter works on both situations where there's a
space after each comma or there's not.

Case-insensitive matching can be performed by ``#/PATTERN/i``:

* List all tags inherits from the class "A" or "a":

  .. code-block:: console

     $ readtags -Q '(#/(^|,) ?A(,|$)/i $inherits)' -l

To include "/" in a pattern, prefix ``\`` to the "/".

NOTE: The above regular expression pattern for inspecting inheritances is just
an example to show how to use ``#/PATTERN/`` expression. Tags file generators
have no consensus about the format of ``inherits:``, e.g., whether there should
be a space after a comma. Even parsers in ctags have no consensus. Noticing the
format of the ``inherits:`` field of specific languages is needed for such
queries.

The expressions ``#/PATTERN/`` and ``#/PATTERN/i`` are for interactive use.
Readtags also offers an alias ``string->regexp``, so ``#/PATTERN/`` is equal to
``(string->regexp "PATTERN")``, and ``#/PATTERN/i`` is equal to
``(string->regexp "PATTERN" :case-fold #t)``. ``string->regexp`` doesn't need
to prefix ``\`` for including "/" in a pattern. ``string->regexp`` may simplify
a client tool building an expression. See also :ref:`ctags-client-tools(7) <ctags-client-tools(7)>` for
building expressions in your tool.

Let's now consider missing fields. The tags file may have tag entries that has
no ``inherits:`` field. In that case ``$inherits`` is #f, and the regular
expression matching raises an error, since string operators only work for
strings. To avoid this problem:

* Safely list all tags inherits from the class "A":

  .. code-block:: console

     $ readtags -Q '(and $inherits (#/(^|,) ?A(,|$)/ $inherits))' -l

This makes sure ``$inherits`` is not missing first, then match it by regexp.

Sometimes you want to keep tags where the field *is* missing. For example, your
want to exclude reference tags, which is marked by the ``extras:`` field, then
you want to keep tags who doesn't have ``extras:`` field since they are also
not reference tags. Here's how to do it:

* List all tags but the reference tags:

  .. code-block:: console

     $ readtags -Q '(or (not $extras) (#/(^|,) ?reference(,|$)/ $extras))' -l

Notice that ``(not $extras)`` produces ``#t`` when ``$extras`` is missing, so
the whole ``or`` expression produces ``#t``.

Run "readtags -H filter" to know about all valid functions and variables.

Sorting
~~~~~~~
When sorting, the sorter expression is evaluated on two tag entries to decide
which should sort before the other one, until the order of all tag entries is
decided.

In a sorter expression, ``$`` and ``&`` are used to access the fields in the
two tag entries, and let's call them $-entry and &-entry. The sorter expression
should have a value of -1, 0 or 1. The value -1 means the $-entry should be put
above the &-entry, 1 means the contrary, and 0 makes their order in the output
uncertain.

The core operator of sorting is ``<>``. It's used to compare two strings or two
numbers (numbers are for the ``line:`` or ``end:`` fields). In ``(<> a b)``, if
``a`` < ``b``, the result is -1; ``a`` > ``b`` produces 1, and ``a`` = ``b``
produces 0. Strings are compared using the ``strcmp`` function, see strcmp(3).

For example, sort by names, and make those shorter or alphabetically smaller
ones appear before the others:

.. code-block:: console

   $ readtags -S '(<> $name &name)' -l

This reads "If the tag name in the $-entry is smaller, it goes before the
&-entry".

The ``<or>`` operator is used to chain multiple expressions until one returns
-1 or 1. For example, sort by input file names, then line numbers if in the
same file:

.. code-block:: console

   $ readtags -S '(<or> (<> $input &input) (<> $line &line))' -l

The ``*-`` operator is used to flip the compare result. i.e., ``(*- (<> a b))``
is the same as ``(<> b a)``.

Filter expressions can be used in sorter expressions. The technique is use
``if`` to produce integers that can be compared based on the filter, like:

.. code-block:: lisp

   (<> (if filter-expr-on-$-entry -1 1)
       (if filter-expr-on-&-entry -1 1))

So if $-entry satisfies the filter, while &-entry doesn't, it's the same as
``(<> -1 1)``, which produces ``-1``.

For example, we want to put tags with "file" kind below other tags, then the
sorter would look like:

.. code-block:: lisp

   (<> (if (eq? $kind "file") 1 -1)
       (if (eq? &kind "file") 1 -1))

A quick read tells us: If $-entry has "file" kind, and &-entry doesn't, the
sorter becomes ``(<> 1 -1)``, which produces ``1``, so the $-entry is put below
the &-entry, exactly what we want.

Inspecting the Behavior of Expressions
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
The `print` operator can be used to print the value of an expression. For
example:

.. code-block:: console

   $ readtags -Q '(print $name)' -l

prints the name of each tag entry before it. Since the return value of
``print`` is not #f, all the tag entries are printed. We could control this
using the ``begin`` or ``begin0`` operator. ``begin`` returns the value of its
last argument, and ``begin0`` returns the value of its first argument. For
example:

.. code-block:: console

   $ readtags -Q '(begin0 #f (print (prefix? "ctags" "ct")))' -l

prints a bunch of "#t" (depending on how many lines are in the tags file), and
the actual tag entries are not printed.

SEE ALSO
--------
See :ref:`tags(5) <tags(5)>` for the details of tags file format.

See :ref:`ctags-client-tools(7) <ctags-client-tools(7)>` for the tips writing a
tool utilizing tags file.

The official Universal Ctags web site at:

https://ctags.io/

The git repository for the library used in readtags command:

https://github.com/universal-ctags/libreadtags

CREDITS
-------
Universal Ctags project
https://ctags.io/

Darren Hiebert <dhiebert@users.sourceforge.net>
http://DarrenHiebert.com/

The readtags command and libreadtags maintained at Universal Ctags
are derived from readtags.c and readtags.h developd at
http://ctags.sourceforge.net.
