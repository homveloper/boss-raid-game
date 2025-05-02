#!/bin/bash
set -e

# Build script for LuvJSON Client SDK

# Configuration
OUTPUT_DIR="./dist"
WASM_DIR="$OUTPUT_DIR/wasm"
JS_DIR="$OUTPUT_DIR/js"
UE_DIR="$OUTPUT_DIR/ue"

# Create output directories
mkdir -p $WASM_DIR $JS_DIR $UE_DIR

echo "Building LuvJSON Client SDK..."

# Build WASM module
echo "Building WASM module..."
GOOS=js GOARCH=wasm go build -o $WASM_DIR/luvjson.wasm ./src/bindings

# Copy WASM exec helper
echo "Copying WASM exec helper..."
cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" $JS_DIR/

# Copy JavaScript wrapper
echo "Copying JavaScript wrapper..."
cp ./src/js/index.js $JS_DIR/luvjson.js

# Create UE plugin structure
echo "Creating UE plugin structure..."
mkdir -p $UE_DIR/LuvJsonCRDT/Source/LuvJsonCRDT/Public
mkdir -p $UE_DIR/LuvJsonCRDT/Source/LuvJsonCRDT/Private
mkdir -p $UE_DIR/LuvJsonCRDT/Resources

# Copy UE header files
echo "Copying UE header files..."
cp ./src/ue/LuvJsonCRDT.h $UE_DIR/LuvJsonCRDT/Source/LuvJsonCRDT/Public/

# Copy WASM files to UE plugin
echo "Copying WASM files to UE plugin..."
mkdir -p $UE_DIR/LuvJsonCRDT/Resources/WASM
cp $WASM_DIR/luvjson.wasm $UE_DIR/LuvJsonCRDT/Resources/WASM/
cp $JS_DIR/wasm_exec.js $UE_DIR/LuvJsonCRDT/Resources/WASM/

# Create UE plugin descriptor
echo "Creating UE plugin descriptor..."
cat > $UE_DIR/LuvJsonCRDT/LuvJsonCRDT.uplugin << EOF
{
	"FileVersion": 3,
	"Version": 1,
	"VersionName": "1.0",
	"FriendlyName": "LuvJSON CRDT",
	"Description": "CRDT-based JSON document synchronization",
	"Category": "Networking",
	"CreatedBy": "Your Company",
	"CreatedByURL": "https://yourcompany.com",
	"DocsURL": "",
	"MarketplaceURL": "",
	"SupportURL": "",
	"CanContainContent": true,
	"IsBetaVersion": true,
	"IsExperimentalVersion": false,
	"Installed": false,
	"Modules": [
		{
			"Name": "LuvJsonCRDT",
			"Type": "Runtime",
			"LoadingPhase": "Default"
		}
	]
}
EOF

# Create UE module build file
echo "Creating UE module build file..."
cat > $UE_DIR/LuvJsonCRDT/Source/LuvJsonCRDT/LuvJsonCRDT.Build.cs << EOF
using UnrealBuildTool;

public class LuvJsonCRDT : ModuleRules
{
	public LuvJsonCRDT(ReadOnlyTargetRules Target) : base(Target)
	{
		PCHUsage = ModuleRules.PCHUsageMode.UseExplicitOrSharedPCHs;
		
		PublicIncludePaths.AddRange(
			new string[] {
				// ... add public include paths required here ...
			}
		);
				
		PrivateIncludePaths.AddRange(
			new string[] {
				// ... add other private include paths required here ...
			}
		);
			
		PublicDependencyModuleNames.AddRange(
			new string[]
			{
				"Core",
				"CoreUObject",
				"Engine",
				"Json",
				"JsonUtilities"
			}
		);
			
		PrivateDependencyModuleNames.AddRange(
			new string[]
			{
				// ... add private dependencies that you statically link with here ...	
			}
		);
		
		DynamicallyLoadedModuleNames.AddRange(
			new string[]
			{
				// ... add any modules that your module loads dynamically here ...
			}
		);
	}
}
EOF

# Create UE module implementation file
echo "Creating UE module implementation file..."
cat > $UE_DIR/LuvJsonCRDT/Source/LuvJsonCRDT/Private/LuvJsonCRDTModule.cpp << EOF
#include "LuvJsonCRDTModule.h"
#include "Modules/ModuleManager.h"

IMPLEMENT_MODULE(FLuvJsonCRDTModule, LuvJsonCRDT)

void FLuvJsonCRDTModule::StartupModule()
{
    // This code will execute after your module is loaded into memory
}

void FLuvJsonCRDTModule::ShutdownModule()
{
    // This function may be called during shutdown to clean up your module
}
EOF

# Create UE module header file
echo "Creating UE module header file..."
cat > $UE_DIR/LuvJsonCRDT/Source/LuvJsonCRDT/Public/LuvJsonCRDTModule.h << EOF
#pragma once

#include "CoreMinimal.h"
#include "Modules/ModuleInterface.h"

class FLuvJsonCRDTModule : public IModuleInterface
{
public:
    /** IModuleInterface implementation */
    virtual void StartupModule() override;
    virtual void ShutdownModule() override;
};
EOF

echo "Build completed successfully!"
echo "Output files are in $OUTPUT_DIR"
