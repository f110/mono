===================
controller-manager
===================

controller-manager is the controller for k8s for homecluster.

Assets
=========

Command
----------

- //go/cmd/controller-manager

Manifest
---------

The manifest for k8s is existing under ``//manifests/deploy/controller-manager`` directory.

Make container
==================

.. code::

    $ bazel run //containers/controller-manager:push

Run locally
=============

.. code::

    $ bazel run //go/cmd/controller-manager --

Run on local cluster
=======================

.. code::

    $ bazel run //go/cmd/controller-manager:run
