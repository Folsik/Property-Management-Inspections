#!/usr/bin/env python3
"""Guard source-of-truth configuration; run after XcodeGen and in CI."""
from pathlib import Path
import re
import subprocess
import sys

spec = Path("project.yml").read_text()
swift = Path("Inspectly/Networking/EndpointConfiguration.swift").read_text()
assert "configs:\n  Debug: debug\n  Staging: release\n  Release: release" in spec
assert "INFOPLIST_KEY_InspectlyAPIBaseURL: \"$(INSPECTLY_API_BASE_URL)\"" in spec
assert "INFOPLIST_KEY_InspectlyEnvironment: \"$(INSPECTLY_ENVIRONMENT)\"" in spec
assert "INSPECTLY_API_BASE_URL: \"\"" in spec
assert "http://localhost:8080" in spec
assert "https://api.example.invalid" not in spec
assert "INFOPLIST_KEY_NSAppTransportSecurity_NSAllowsLocalNetworking: YES" in spec
assert "scheme == \"https\"" in swift and "environment == \"production\"" in swift
assert "return nil" in swift
assert "PRODUCT_BUNDLE_IDENTIFIER: com.inspectly.inspections" in spec
assert "NSCameraUsageDescription" in spec and "NSMicrophoneUsageDescription" in spec

for configuration, env in (("Debug", "local"), ("Staging", "staging"), ("Release", "production")):
    output = subprocess.check_output([
        "xcodebuild", "-project", "Inspectly.xcodeproj", "-scheme", "Inspectly",
        "-configuration", configuration, "-showBuildSettings",
        "CODE_SIGNING_ALLOWED=NO",
    ], text=True)
    values = dict(re.findall(r"^\s+(\w+) = (.*)$", output, re.MULTILINE))
    assert values.get("INSPECTLY_ENVIRONMENT") == env, (configuration, values.get("INSPECTLY_ENVIRONMENT"))
    expected = "http://localhost:8080" if configuration == "Debug" else ""
    assert values.get("INSPECTLY_API_BASE_URL", "") == expected, configuration
    if configuration != "Debug":
        assert values.get("INFOPLIST_KEY_NSAppTransportSecurity_NSAllowsLocalNetworking") != "YES", configuration
print("Environment/ATS/endpoint build-setting checks passed")
