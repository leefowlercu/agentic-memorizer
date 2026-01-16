==================
Sample RST Document
==================

This is a sample reStructuredText document for testing the RST chunker.

Introduction
============

reStructuredText is a lightweight markup language designed to be both:

1. Readable by humans in source form
2. Easily processed by documentation tools

Key Features
------------

The format supports various text formatting options:

* **Bold text** using double asterisks
* *Italic text* using single asterisks
* ``inline code`` using double backticks

Code Blocks
~~~~~~~~~~~

Here is an example code block:

.. code-block:: python

    def hello_world():
        """A simple greeting function."""
        print("Hello, World!")
        return True

    if __name__ == "__main__":
        hello_world()

Nested Section
^^^^^^^^^^^^^^

This demonstrates deeply nested sections with various underline styles.
RST determines heading levels by the order of first appearance.

Installation
============

To install the package, run the following command:

.. code-block:: bash

    pip install mypackage

Configuration
-------------

Configuration can be done via a YAML file:

.. code-block:: yaml

    settings:
      debug: true
      log_level: info

Environment Variables
~~~~~~~~~~~~~~~~~~~~~

You can also use environment variables:

``MY_APP_DEBUG``
    Enable debug mode (default: false)

``MY_APP_LOG_LEVEL``
    Set logging level (default: warning)

API Reference
=============

This section documents the public API.

Classes
-------

.. py:class:: MyClass(param1, param2)

   A sample class for demonstration.

   :param param1: The first parameter
   :param param2: The second parameter

Methods
~~~~~~~

.. py:method:: MyClass.do_something()

   Performs an important action.

   :returns: True on success
   :rtype: bool

Troubleshooting
===============

Common Issues
-------------

Problem: Installation Fails
~~~~~~~~~~~~~~~~~~~~~~~~~~~

If installation fails, try:

1. Updating pip: ``pip install --upgrade pip``
2. Installing build dependencies
3. Checking Python version compatibility

Problem: Import Errors
~~~~~~~~~~~~~~~~~~~~~~

Make sure the package is installed in the correct environment.

Advanced Topics
===============

Custom Extensions
-----------------

You can create custom directives:

.. code-block:: python

    from docutils.parsers.rst import Directive

    class MyDirective(Directive):
        def run(self):
            return []

Performance Tuning
------------------

Tips for optimizing performance:

* Use caching for expensive operations
* Enable lazy loading where possible
* Profile before optimizing

Conclusion
==========

This document demonstrates various RST features for testing purposes.
The heading hierarchy uses different underline characters to denote levels.
