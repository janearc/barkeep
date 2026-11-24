package main

/*
#cgo LDFLAGS: -framework IOKit -framework CoreFoundation
#include <IOKit/IOKitLib.h>
#include <CoreFoundation/CoreFoundation.h>

// barkeep_use is one accelerator's own count of how busy it is, as a
// percentage, or -1 when it does not say.
static int barkeep_use(io_object_t each) {
	CFTypeRef held = IORegistryEntryCreateCFProperty(each,
		CFSTR("PerformanceStatistics"), kCFAllocatorDefault, 0);
	if (held == NULL) {
		return -1;
	}
	int use = -1;
	long long value;
	if (CFGetTypeID(held) == CFDictionaryGetTypeID()) {
		CFTypeRef share = CFDictionaryGetValue(held,
			CFSTR("Device Utilization %"));
		int counts = share != NULL
			&& CFGetTypeID(share) == CFNumberGetTypeID();
		if (counts && CFNumberGetValue(share, kCFNumberLongLongType,
				&value)) {
			use = (int)value;
		}
	}
	CFRelease(held);
	return use;
}

// barkeep_gpu_busy is the busiest graphics accelerator's count of how
// busy it is, as a percentage, or -1 when none says.
static int barkeep_gpu_busy(void) {
	io_iterator_t found;
	CFMutableDictionaryRef wanted = IOServiceMatching("IOAccelerator");
	if (IOServiceGetMatchingServices(kIOMainPortDefault, wanted,
			&found) != KERN_SUCCESS) {
		return -1;
	}
	int busiest = -1;
	io_object_t each;
	while ((each = IOIteratorNext(found))) {
		int use = barkeep_use(each);
		if (use > busiest) {
			busiest = use;
		}
		IOObjectRelease(each);
	}
	IOObjectRelease(found);
	return busiest;
}
*/
import "C"

// gpuBusy is how busy the GPU says it is this moment, as a percentage,
// and whether it said: the figure the graphics accelerator publishes in
// the I/O registry as "Device Utilization %". With more than one
// accelerator it is the busiest.
func gpuBusy() (int, bool) {
	busy := int(C.barkeep_gpu_busy())
	return busy, busy >= 0
}
