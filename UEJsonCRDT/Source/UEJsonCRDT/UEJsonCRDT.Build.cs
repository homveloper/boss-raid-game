// Copyright Your Company. All Rights Reserved.

using UnrealBuildTool;
using System.IO;

public class UEJsonCRDT : ModuleRules
{
	public UEJsonCRDT(ReadOnlyTargetRules Target) : base(Target)
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
				"InputCore",
				"Json",
				"JsonUtilities",
				"HTTP",
				"WebSockets",
				// ... add other public dependencies that you statically link with here ...
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
		
		// Add the CRDT library headers
		string ThirdPartyPath = Path.Combine(ModuleDirectory, "../../ThirdParty");
		PublicIncludePaths.Add(Path.Combine(ThirdPartyPath, "luvjson/include"));
	}
}
