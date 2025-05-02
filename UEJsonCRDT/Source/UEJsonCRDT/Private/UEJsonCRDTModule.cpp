// Copyright Your Company. All Rights Reserved.

#include "UEJsonCRDTModule.h"
#include "Modules/ModuleManager.h"
#include "Interfaces/IPluginManager.h"

#define LOCTEXT_NAMESPACE "FUEJsonCRDTModule"

void FUEJsonCRDTModule::StartupModule()
{
	// This code will execute after your module is loaded into memory; the exact timing is specified in the .uplugin file per-module
	UE_LOG(LogTemp, Log, TEXT("UEJsonCRDT module has started"));
}

void FUEJsonCRDTModule::ShutdownModule()
{
	// This function may be called during shutdown to clean up your module.  For modules that support dynamic reloading,
	// we call this function before unloading the module.
	UE_LOG(LogTemp, Log, TEXT("UEJsonCRDT module has shut down"));
}

#undef LOCTEXT_NAMESPACE
	
IMPLEMENT_MODULE(FUEJsonCRDTModule, UEJsonCRDT)
